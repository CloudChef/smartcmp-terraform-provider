package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDeploymentActionsRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/platform-api/login" {
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path != "/platform-api/deployments/dep-1/deployment-actions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if cookie, _ := r.Cookie("SESSION"); cookie == nil {
			t.Fatalf("expected authenticated request")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{
				"id":                   "restart",
				"name":                 "Restart",
				"description":          "Restart the deployment",
				"enabled":              true,
				"mfa":                  true,
				"supportCharge":        false,
				"supportScheduledTask": true,
				"supportBatchAction":   true,
				"actionCategory": map[string]any{
					"name":  "Lifecycle",
					"order": 3,
				},
				"parameters": `{"fields":[{"name":"force"}]}`,
			},
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &DeploymentActionsDataSource{client: apiClient}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"deployment_id": tfStringValue("dep-1"),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state DeploymentActionsDataSourceModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state decode failed: %v", diags)
	}
	var items []OperationActionItemModel
	if diags := state.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatalf("items decode failed: %v", diags)
	}
	if len(items) != 1 {
		t.Fatalf("expected one action, got %d", len(items))
	}
	if items[0].Operation.ValueString() != "restart" {
		t.Fatalf("unexpected operation: %s", items[0].Operation.ValueString())
	}
	if items[0].Name.ValueString() != "Restart" {
		t.Fatalf("unexpected name: %s", items[0].Name.ValueString())
	}
	if items[0].ActionCategoryName.ValueString() != "Lifecycle" {
		t.Fatalf("unexpected action category: %s", items[0].ActionCategoryName.ValueString())
	}
	if items[0].ActionCategoryOrder.ValueInt64() != 3 {
		t.Fatalf("unexpected action category order: %d", items[0].ActionCategoryOrder.ValueInt64())
	}
	if items[0].SchemaJSON.ValueString() != `{"fields":[{"name":"force"}]}` {
		t.Fatalf("unexpected schema_json: %s", items[0].SchemaJSON.ValueString())
	}
}
