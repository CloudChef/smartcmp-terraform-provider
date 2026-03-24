package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestResourceActionsByIDsRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/platform-api/login" {
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path != "/platform-api/nodes/iaas.machine/batch/resource-actions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}
		if string(body) != `{"ids":["res-1","res-2"]}` {
			t.Fatalf("unexpected request body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"res-1": []any{
				map[string]any{
					"id":                   "restart",
					"name":                 "Restart",
					"supportScheduledTask": true,
					"supportBatchAction":   true,
					"parameters":           `{"fields":[{"name":"graceful"}]}`,
				},
			},
			"res-2": []any{
				map[string]any{
					"id":                 "powerOff",
					"name":               "Power Off",
					"supportBatchAction": true,
				},
				map[string]any{
					"id":           "resize",
					"name":         "Resize",
					"webOperation": true,
				},
			},
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &ResourceActionsByIDsDataSource{client: apiClient}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"resource_category": tfStringValue("iaas.machine"),
		"resource_ids":      tfStringListValue("res-1", "res-2"),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ResourceActionsByIDsDataSourceModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state decode failed: %v", diags)
	}
	if state.TotalResources.ValueInt64() != 2 {
		t.Fatalf("unexpected total_resources: %d", state.TotalResources.ValueInt64())
	}

	var items []ResourceActionSetItemModel
	if diags := state.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatalf("items decode failed: %v", diags)
	}
	if len(items) != 2 {
		t.Fatalf("expected two resource action sets, got %d", len(items))
	}
	if items[0].ResourceID.ValueString() != "res-1" || items[1].ResourceID.ValueString() != "res-2" {
		t.Fatalf("unexpected resource order: %s, %s", items[0].ResourceID.ValueString(), items[1].ResourceID.ValueString())
	}

	var first []OperationActionItemModel
	if diags := items[0].Actions.ElementsAs(ctx, &first, false); diags.HasError() {
		t.Fatalf("first actions decode failed: %v", diags)
	}
	if len(first) != 1 || first[0].Operation.ValueString() != "restart" {
		t.Fatalf("unexpected first resource actions: %+v", first)
	}

	var second []OperationActionItemModel
	if diags := items[1].Actions.ElementsAs(ctx, &second, false); diags.HasError() {
		t.Fatalf("second actions decode failed: %v", diags)
	}
	if len(second) != 2 {
		t.Fatalf("expected two second-resource actions, got %d", len(second))
	}
	if !second[1].WebOperation.ValueBool() {
		t.Fatalf("expected resize action to be marked as web_operation")
	}
}

func TestResourceActionsByIDsReadRejectsEmptyIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds := &ResourceActionsByIDsDataSource{}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"resource_category": tfStringValue("iaas.machine"),
		"resource_ids":      tfStringListValue(),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected diagnostics for empty resource_ids")
	}
}
