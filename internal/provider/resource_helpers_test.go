package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExtractTaskFromBatchResponse(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"results": map[string]any{
			"resource-1": map[string]any{
				"id":          "task-1",
				"state":       "CREATED",
				"resourceIds": []any{"resource-1"},
			},
		},
		"webOperation": false,
	}

	task, err := extractTaskFromBatchResponse(raw, "resource-1")
	if err != nil {
		t.Fatalf("extractTaskFromBatchResponse returned error: %v", err)
	}

	if got := findFirstString(task, "id"); got != "task-1" {
		t.Fatalf("expected task id %q, got %q", "task-1", got)
	}
}

func TestExtractTaskFromBatchResponseRejectsWebOperation(t *testing.T) {
	t.Parallel()

	_, err := extractTaskFromBatchResponse(map[string]any{
		"results":      map[string]any{},
		"webOperation": true,
	}, "resource-1")
	if err == nil {
		t.Fatal("expected error for webOperation response")
	}
}

func TestTaskIsTerminal(t *testing.T) {
	t.Parallel()

	if !taskIsTerminal("FINISHED") {
		t.Fatal("expected FINISHED to be terminal")
	}
	if taskIsTerminal("SUBMITTED") {
		t.Fatal("expected SUBMITTED to remain non-terminal")
	}
}

func TestApplyServiceRequestRaw(t *testing.T) {
	t.Parallel()

	var data ServiceRequestResourceModel
	applyServiceRequestRaw(&data, map[string]any{
		"id":              "req-1",
		"catalogId":       "catalog-1",
		"businessGroupId": "bg-1",
		"name":            "linux-vm",
		"state":           "STARTED",
		"errMsg":          "boom",
		"completedDate":   float64(1_700_000_000_000),
		"requestUserId":   "user-1",
		"inventoryId":     "inventory-1",
		"objectId":        "object-1",
		"objectType":      "deployment",
	})

	if data.ID.ValueString() != "req-1" {
		t.Fatalf("expected id to be populated, got %q", data.ID.ValueString())
	}
	if data.CatalogID.ValueString() != "catalog-1" || data.BusinessGroupID.ValueString() != "bg-1" {
		t.Fatalf("expected catalog/business group fields to be populated, got %+v", data)
	}
	if data.CompletedAt.IsNull() || data.CompletedAt.ValueString() == "" {
		t.Fatal("expected completed_at to be populated")
	}
}

func TestApplyServiceRequestRawPreservesConfiguredInputs(t *testing.T) {
	t.Parallel()

	data := ServiceRequestResourceModel{
		CatalogID:       types.StringValue("catalog-config"),
		BusinessGroupID: types.StringValue("bg-config"),
		Name:            types.StringValue("linux-config"),
		Description:     types.StringValue("description-config"),
		ProjectID:       types.StringValue("project-config"),
		RequestUserID:   types.StringValue("user-config"),
		ResourcePoolID:  types.StringValue("pool-config"),
	}

	applyServiceRequestRaw(&data, map[string]any{
		"id":               "req-1",
		"catalogId":        "catalog-raw",
		"businessGroupId":  "bg-raw",
		"name":             "linux-raw",
		"description":      "description-raw",
		"projectId":        "project-raw",
		"requestUserId":    "user-raw",
		"resourceBundleId": "pool-raw",
		"state":            "FINISHED",
	})

	if got := data.CatalogID.ValueString(); got != "catalog-config" {
		t.Fatalf("expected catalog_id to keep configured value, got %q", got)
	}
	if got := data.BusinessGroupID.ValueString(); got != "bg-config" {
		t.Fatalf("expected business_group_id to keep configured value, got %q", got)
	}
	if got := data.Name.ValueString(); got != "linux-config" {
		t.Fatalf("expected name to keep configured value, got %q", got)
	}
	if got := data.Description.ValueString(); got != "description-config" {
		t.Fatalf("expected description to keep configured value, got %q", got)
	}
	if got := data.ProjectID.ValueString(); got != "project-config" {
		t.Fatalf("expected project_id to keep configured value, got %q", got)
	}
	if got := data.RequestUserID.ValueString(); got != "user-config" {
		t.Fatalf("expected request_user_id to keep configured value, got %q", got)
	}
	if got := data.ResourcePoolID.ValueString(); got != "pool-config" {
		t.Fatalf("expected resource_pool_id to keep configured value, got %q", got)
	}
}

func TestPopulateServiceRequestDeploymentIDsWithoutRequestID(t *testing.T) {
	t.Parallel()

	data := ServiceRequestResourceModel{ID: types.StringNull()}
	warning, diags := populateServiceRequestDeploymentIDs(context.Background(), nil, &data)
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if data.DeploymentIDs.IsNull() {
		t.Fatal("expected deployment_ids to be initialized to an empty list")
	}
}
