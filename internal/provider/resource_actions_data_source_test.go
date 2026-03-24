package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestResourceActionsReadSingleResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/platform-api/login" {
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path != "/platform-api/nodes/iaas.machine/res-1/resource-actions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("taskType") != "PIPELINE" {
			t.Fatalf("unexpected query: %v", r.URL.Query())
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{
				"id":                   "powerOn",
				"name":                 "Power On",
				"enabled":              true,
				"disabledMsg":          "",
				"supportScheduledTask": true,
				"supportBatchAction":   false,
				"supportCharge":        false,
				"webOperation":         false,
				"inputsForm": map[string]any{
					"fields": []any{
						map[string]any{"name": "force", "type": "boolean"},
					},
				},
			},
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &ResourceActionsDataSource{client: apiClient}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"resource_category": tfStringValue("iaas.machine"),
		"resource_id":       tfStringValue("res-1"),
		"task_type":         tfStringValue("PIPELINE"),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state ResourceActionsDataSourceModel
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
	if items[0].Operation.ValueString() != "powerOn" {
		t.Fatalf("unexpected operation: %s", items[0].Operation.ValueString())
	}
	if !strings.Contains(items[0].SchemaJSON.ValueString(), `"force"`) {
		t.Fatalf("unexpected schema_json: %s", items[0].SchemaJSON.ValueString())
	}
}

func TestResourceActionsReadCommonBatchActions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/platform-api/login" {
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path != "/platform-api/nodes/iaas.machine/resource-actions" {
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
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{
				"id":                 "restart",
				"name":               "Restart",
				"enabled":            true,
				"supportBatchAction": true,
				"parameters":         `{"fields":[{"name":"graceful"}]}`,
			},
		})
	})
	defer server.Close()

	apiClient := newLoggedInTestClient(t, server.URL, testUsername, testPassword, testTenantID, true)
	ds := &ResourceActionsDataSource{client: apiClient}

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

	var state ResourceActionsDataSourceModel
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
	if !items[0].SupportBatchAction.ValueBool() {
		t.Fatalf("expected support_batch_action to be true")
	}
}

func TestResourceActionsReadRejectsConflictingSelectors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds := &ResourceActionsDataSource{}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	req := newDataSourceReadRequest(t, schemaResp.Schema, map[string]tftypes.Value{
		"resource_category": tfStringValue("iaas.machine"),
		"resource_id":       tfStringValue("res-1"),
		"resource_ids":      tfStringListValue("res-1", "res-2"),
	})
	resp := newDataSourceReadResponse(t, schemaResp.Schema)
	ds.Read(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected diagnostics for conflicting selectors")
	}
}
