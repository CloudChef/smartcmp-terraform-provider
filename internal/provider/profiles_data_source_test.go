package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProfilesReadUsesProvisionScopeAndNormalizesDimensions(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		provisionScope bool
		wantPath       string
	}{
		"default provision scope": {
			provisionScope: true,
			wantPath:       "/platform-api/flavors/provision",
		},
		"non-provision scope": {
			provisionScope: false,
			wantPath:       "/platform-api/flavors",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			var gotPath string
			var gotQuery url.Values
			var mu sync.Mutex

			server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/platform-api/login" {
					http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
					w.WriteHeader(http.StatusOK)
					return
				}

				mu.Lock()
				gotPath = r.URL.Path
				gotQuery = captureQuery(r.URL.Query())
				mu.Unlock()
				if cookie, _ := r.Cookie("SESSION"); cookie == nil {
					t.Fatalf("expected authenticated request")
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"content": []any{
						map[string]any{
							"id":                  "profile-1",
							"name":                "Small",
							"nameEn":              "Small",
							"description":         "1 CPU, 2 GB",
							"specType":            "MACHINE",
							"flavorType":          "MACHINE",
							"changeWhenProvision": true,
							"matchCpuAndMemory":   false,
							"weight":              2.5,
							"flavors": []any{
								map[string]any{"type": "cpu", "number": 1},
								map[string]any{"type": "memory", "number": 2},
							},
						},
					},
					"totalElements": 1,
				})
			})
			defer server.Close()

			apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
			ds := &ProfilesDataSource{client: apiClient}

			var schemaResp datasource.SchemaResponse
			ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

			req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
				"query":               tfStringValue("Small"),
				"flavor_type":         tfStringValue("MACHINE"),
				"spec_type":           tfStringValue("MACHINE"),
				"catalog_id":          tfStringValue("BUILD-IN-CATALOG-LINUX-VM"),
				"node_template_name":  tfStringValue("Compute"),
				"resource_bundle_id":  tfNullStringValue(),
				"cloud_entry_type_id": tfNullStringValue(),
				"provision_scope":     tfBoolValue(tc.provisionScope),
			})
			resp := newDataSourceReadResponse(t, schemaResp.Schema)
			ds.Read(ctx, req, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			mu.Lock()
			path := gotPath
			query := gotQuery
			mu.Unlock()
			if path != tc.wantPath {
				t.Fatalf("unexpected path: %s", path)
			}
			if got := query.Get("queryValue"); got != "Small" {
				t.Fatalf("unexpected queryValue: %v", query)
			}
			if got := query.Get("flavorType"); got != "MACHINE" {
				t.Fatalf("unexpected flavorType: %v", query)
			}
			if got := query.Get("catalogId"); got != "BUILD-IN-CATALOG-LINUX-VM" {
				t.Fatalf("unexpected catalogId: %v", query)
			}
			if got := query.Get("nodeTemplateName"); got != "Compute" {
				t.Fatalf("unexpected nodeTemplateName: %v", query)
			}

			var state ProfilesDataSourceModel
			if diags := resp.State.Get(ctx, &state); diags.HasError() {
				t.Fatalf("state decode failed: %v", diags)
			}
			var items []ProfileItemModel
			if diags := state.Items.ElementsAs(ctx, &items, false); diags.HasError() {
				t.Fatalf("items decode failed: %v", diags)
			}
			if len(items) != 1 {
				t.Fatalf("expected one profile, got %d", len(items))
			}
			if items[0].CPU.ValueInt64() != 1 || items[0].MemoryGB.ValueInt64() != 2 {
				t.Fatalf("unexpected CPU/memory: %d/%d", items[0].CPU.ValueInt64(), items[0].MemoryGB.ValueInt64())
			}
			if items[0].Name.ValueString() != "Small" || items[0].NameEn.ValueString() != "Small" {
				t.Fatalf("unexpected profile names: %s/%s", items[0].Name.ValueString(), items[0].NameEn.ValueString())
			}
		})
	}
}
