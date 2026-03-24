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
