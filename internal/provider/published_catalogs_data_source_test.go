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

func TestPublishedCatalogsReadBuildsQueryAndMapsItems(t *testing.T) {
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
					"id":               "catalog-1",
					"nameZh":           "Linux VM",
					"sourceKey":        "BUILD-IN-CATALOG-LINUX-VM",
					"serviceCategory":  "Compute",
					"instructions":     "Linux VM catalog",
					"additionalFields": map[string]any{"ignored": true},
				},
			},
			"totalElements": 1,
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &PublishedCatalogsDataSource{client: apiClient}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"query": tfStringValue("Linux VM"),
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
	if path != "/platform-api/catalogs/published" {
		t.Fatalf("unexpected path: %s", path)
	}
	if got := query.Get("states"); got != "PUBLISHED" {
		t.Fatalf("unexpected states query: %v", query)
	}
	if got := query.Get("queryValue"); got != "Linux VM" {
		t.Fatalf("unexpected queryValue: %v", query)
	}

	var state PublishedCatalogsDataSourceModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state decode failed: %v", diags)
	}
	if state.Total.ValueInt64() != 1 {
		t.Fatalf("expected total 1, got %d", state.Total.ValueInt64())
	}

	var items []PublishedCatalogItemModel
	if diags := state.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatalf("items decode failed: %v", diags)
	}
	if len(items) != 1 {
		t.Fatalf("expected one catalog, got %d", len(items))
	}
	if items[0].SourceKey.ValueString() != "BUILD-IN-CATALOG-LINUX-VM" {
		t.Fatalf("unexpected source key: %s", items[0].SourceKey.ValueString())
	}
}
