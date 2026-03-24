package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCatalogBusinessGroupsReadUsesCatalogPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var gotPath string
	var mu sync.Mutex

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/platform-api/login" {
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}

		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()
		if cookie, _ := r.Cookie("SESSION"); cookie == nil {
			t.Fatalf("expected authenticated request")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []any{
				map[string]any{
					"id":          "bg-1",
					"name":        "Default",
					"description": "Default business group",
				},
			},
			"totalElements": 1,
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &CatalogBusinessGroupsDataSource{client: apiClient}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"catalog_id": tfStringValue("BUILD-IN-CATALOG-LINUX-VM"),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	mu.Lock()
	path := gotPath
	mu.Unlock()
	if path != "/platform-api/catalogs/BUILD-IN-CATALOG-LINUX-VM/available-bgs" {
		t.Fatalf("unexpected path: %s", path)
	}

	var state CatalogBusinessGroupsDataSourceModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state decode failed: %v", diags)
	}
	if state.Total.ValueInt64() != 1 {
		t.Fatalf("expected total 1, got %d", state.Total.ValueInt64())
	}
	if !state.Items.IsNull() {
		var items []CatalogBusinessGroupItemModel
		if diags := state.Items.ElementsAs(ctx, &items, false); diags.HasError() {
			t.Fatalf("items decode failed: %v", diags)
		}
		if len(items) != 1 {
			t.Fatalf("expected one business group, got %d", len(items))
		}
		if items[0].ID.ValueString() != "bg-1" {
			t.Fatalf("unexpected business group ID: %s", items[0].ID.ValueString())
		}
	}
}
