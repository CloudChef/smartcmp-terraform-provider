package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildServiceRequestPayloadOverlaysTypedRequestFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	payload, err := buildServiceRequestPayload(ctx, ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-typed"),
		BusinessGroupID:        types.StringValue("bg-typed"),
		Name:                   types.StringValue("name-typed"),
		Description:            types.StringValue("description-typed"),
		ProjectID:              types.StringValue("project-typed"),
		RequestUserID:          types.StringValue("user-typed"),
		Count:                  types.Int64Value(2),
		AddToInventory:         types.BoolValue(true),
		ResourcePoolID:         types.StringValue("rb-top"),
		ResourcePoolTags:       mustStringListValue(t, "env:prod"),
		ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-a"}`),
		RequestBodyJSON:        types.StringValue(`{"catalogId":"catalog-json","businessGroupId":"bg-json","name":"name-json","description":"description-json","projectId":"project-json","userId":"user-json","count":99,"addToInventory":false,"exts":{"keep":"me"}}`),
	})
	if err != nil {
		t.Fatalf("buildServiceRequestPayload returned error: %v", err)
	}

	if got := stringValue(payload["catalogId"]); got != "catalog-typed" {
		t.Fatalf("expected typed catalogId, got %q", got)
	}
	if got := stringValue(payload["businessGroupId"]); got != "bg-typed" {
		t.Fatalf("expected typed businessGroupId, got %q", got)
	}
	if got := stringValue(payload["name"]); got != "name-typed" {
		t.Fatalf("expected typed name, got %q", got)
	}
	if got := stringValue(payload["description"]); got != "description-typed" {
		t.Fatalf("expected typed description, got %q", got)
	}
	if got := stringValue(payload["projectId"]); got != "project-typed" {
		t.Fatalf("expected typed projectId, got %q", got)
	}
	if got := stringValue(payload["groupId"]); got != "project-typed" {
		t.Fatalf("expected groupId compatibility mapping, got %q", got)
	}
	if got := stringValue(payload["userId"]); got != "user-typed" {
		t.Fatalf("expected typed userId, got %q", got)
	}
	if got := stringValue(payload["requestUserId"]); got != "user-typed" {
		t.Fatalf("expected requestUserId compatibility mapping, got %q", got)
	}
	if got := numberValue(payload["count"]); got != 2 {
		t.Fatalf("expected typed count 2, got %v", payload["count"])
	}
	if got := boolValue(payload["addToInventory"]); !got {
		t.Fatalf("expected addToInventory true, got %v", payload["addToInventory"])
	}
	if got := stringValue(payload["resourceBundleId"]); got != "rb-top" {
		t.Fatalf("expected top-level resourceBundleId default, got %q", got)
	}
	if got := stringSliceFromAny(payload["resourceBundleTags"]); len(got) != 1 || got[0] != "env:prod" {
		t.Fatalf("expected top-level resourceBundleTags, got %#v", payload["resourceBundleTags"])
	}
	if got := asMap(payload["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-a" {
		t.Fatalf("expected top-level resourceBundleParams to be applied, got %#v", payload["resourceBundleParams"])
	}
	if got := findStringPath(payload, "exts", "keep"); got != "me" {
		t.Fatalf("expected unknown request_body_json fields to be preserved, got %q", got)
	}
}

func TestBuildServiceRequestPayloadBuildsTypedResourceSpecs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	payload, err := buildServiceRequestPayload(ctx, ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-1"),
		BusinessGroupID:        types.StringValue("bg-1"),
		Name:                   types.StringValue("typed"),
		ResourcePoolID:         types.StringValue("rb-top"),
		ResourcePoolTags:       mustStringListValue(t, "env:top"),
		ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-top"}`),
		ResourceSpecs: mustResourceSpecsList(t, []serviceRequestResourceSpecModel{{
			Node:                   types.StringValue("Compute"),
			Type:                   types.StringValue("cloudchef.nodes.Compute"),
			ResourcePoolID:         types.StringValue("rb-spec"),
			ResourcePoolTags:       mustStringListValue(t, "env:spec"),
			ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-spec"}`),
			SpecJSON:               types.StringValue(`{"resourceBundleId":"rb-json","computeProfileId":"cp-1","params":{"k":"v"}}`),
		}}),
	})
	if err != nil {
		t.Fatalf("buildServiceRequestPayload returned error: %v", err)
	}

	rawSpecs, ok := payload["resourceSpecs"].([]map[string]any)
	if !ok {
		t.Fatalf("expected typed resourceSpecs to be []map[string]any, got %#v", payload["resourceSpecs"])
	}
	if len(rawSpecs) != 1 {
		t.Fatalf("expected one resource spec, got %d", len(rawSpecs))
	}

	spec := rawSpecs[0]
	if got := stringValue(spec["node"]); got != "Compute" {
		t.Fatalf("expected node Compute, got %q", got)
	}
	if got := stringValue(spec["type"]); got != "cloudchef.nodes.Compute" {
		t.Fatalf("expected type cloudchef.nodes.Compute, got %q", got)
	}
	if got := stringValue(spec["resourceBundleId"]); got != "rb-spec" {
		t.Fatalf("expected typed spec resourceBundleId override, got %q", got)
	}
	if got := stringSliceFromAny(spec["resourceBundleTags"]); len(got) != 1 || got[0] != "env:spec" {
		t.Fatalf("expected typed spec resourceBundleTags override, got %#v", spec["resourceBundleTags"])
	}
	if got := asMap(spec["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-spec" {
		t.Fatalf("expected typed spec resourceBundleParams override, got %#v", spec["resourceBundleParams"])
	}
	if got := stringValue(spec["computeProfileId"]); got != "cp-1" {
		t.Fatalf("expected spec_json fields to be preserved, got %q", got)
	}
	if got := findStringPath(spec, "params", "k"); got != "v" {
		t.Fatalf("expected spec_json params to be preserved, got %q", got)
	}
}

func TestBuildServiceRequestPayloadFallsBackToTopLevelDefaultsForTypedResourceSpecs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	payload, err := buildServiceRequestPayload(ctx, ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-1"),
		BusinessGroupID:        types.StringValue("bg-1"),
		Name:                   types.StringValue("typed"),
		ResourcePoolID:         types.StringValue("rb-top"),
		ResourcePoolTags:       mustStringListValue(t, "env:top"),
		ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-top"}`),
		ResourceSpecs: mustResourceSpecsList(t, []serviceRequestResourceSpecModel{{
			Node:             types.StringValue("Compute"),
			Type:             types.StringValue("cloudchef.nodes.Compute"),
			ResourcePoolTags: mustStringListValue(t),
			SpecJSON:         types.StringValue(`{"computeProfileId":"cp-1"}`),
		}}),
	})
	if err != nil {
		t.Fatalf("buildServiceRequestPayload returned error: %v", err)
	}

	rawSpecs, ok := payload["resourceSpecs"].([]map[string]any)
	if !ok || len(rawSpecs) != 1 {
		t.Fatalf("expected one typed resourceSpec, got %#v", payload["resourceSpecs"])
	}

	spec := rawSpecs[0]
	if got := stringValue(spec["resourceBundleId"]); got != "rb-top" {
		t.Fatalf("expected top-level resourceBundleId fallback, got %q", got)
	}
	if got := stringSliceFromAny(spec["resourceBundleTags"]); len(got) != 1 || got[0] != "env:top" {
		t.Fatalf("expected top-level resourceBundleTags fallback, got %#v", spec["resourceBundleTags"])
	}
	if got := asMap(spec["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-top" {
		t.Fatalf("expected top-level resourceBundleParams fallback, got %#v", spec["resourceBundleParams"])
	}
}

func TestBuildServiceRequestPayloadFillsResourceDefaultsOnRawJSONSpecs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	payload, err := buildServiceRequestPayload(ctx, ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-1"),
		BusinessGroupID:        types.StringValue("bg-1"),
		Name:                   types.StringValue("typed"),
		ResourcePoolID:         types.StringValue("rb-top"),
		ResourcePoolTags:       mustStringListValue(t, "env:top"),
		ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-top"}`),
		RequestBodyJSON:        types.StringValue(`{"resourceSpecs":[{"node":"Compute","type":"cloudchef.nodes.Compute","params":{"k":"v"}}]}`),
	})
	if err != nil {
		t.Fatalf("buildServiceRequestPayload returned error: %v", err)
	}

	rawSpecs, ok := payload["resourceSpecs"].([]any)
	if !ok || len(rawSpecs) != 1 {
		t.Fatalf("expected raw JSON resourceSpecs to be []any with one item, got %#v", payload["resourceSpecs"])
	}

	spec := asMap(rawSpecs[0])
	if got := stringValue(spec["resourceBundleId"]); got != "rb-top" {
		t.Fatalf("expected top defaults to fill raw resourceSpecs resourceBundleId, got %q", got)
	}
	if got := stringSliceFromAny(spec["resourceBundleTags"]); len(got) != 1 || got[0] != "env:top" {
		t.Fatalf("expected top defaults to fill raw resourceSpecs tags, got %#v", spec["resourceBundleTags"])
	}
	if got := asMap(spec["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-top" {
		t.Fatalf("expected top defaults to fill raw resourceSpecs params, got %#v", spec["resourceBundleParams"])
	}
}

func TestBuildServiceRequestPayloadDoesNotOverrideExplicitRawJSONSpecDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	payload, err := buildServiceRequestPayload(ctx, ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-1"),
		BusinessGroupID:        types.StringValue("bg-1"),
		Name:                   types.StringValue("typed"),
		ResourcePoolID:         types.StringValue("rb-top"),
		ResourcePoolTags:       mustStringListValue(t, "env:top"),
		ResourcePoolParamsJSON: types.StringValue(`{"available_zone_id":"zone-top"}`),
		RequestBodyJSON: types.StringValue(`{
			"resourceSpecs":[
				{
					"node":"Compute",
					"type":"cloudchef.nodes.Compute",
					"resourceBundleId":"rb-explicit",
					"resourceBundleTags":["env:explicit"],
					"resourceBundleParams":{"available_zone_id":"zone-explicit"}
				}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("buildServiceRequestPayload returned error: %v", err)
	}

	rawSpecs, ok := payload["resourceSpecs"].([]any)
	if !ok || len(rawSpecs) != 1 {
		t.Fatalf("expected raw JSON resourceSpecs to be []any with one item, got %#v", payload["resourceSpecs"])
	}

	spec := asMap(rawSpecs[0])
	if got := stringValue(spec["resourceBundleId"]); got != "rb-explicit" {
		t.Fatalf("expected explicit raw resourceBundleId to survive, got %q", got)
	}
	if got := stringSliceFromAny(spec["resourceBundleTags"]); len(got) != 1 || got[0] != "env:explicit" {
		t.Fatalf("expected explicit raw resourceBundleTags to survive, got %#v", spec["resourceBundleTags"])
	}
	if got := asMap(spec["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-explicit" {
		t.Fatalf("expected explicit raw resourceBundleParams to survive, got %#v", spec["resourceBundleParams"])
	}
}

func TestBuildServiceRequestPayloadRejectsInvalidRequestBodyJSON(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequestPayload(context.Background(), ServiceRequestResourceModel{
		CatalogID:       types.StringValue("catalog-1"),
		BusinessGroupID: types.StringValue("bg-1"),
		Name:            types.StringValue("typed"),
		RequestBodyJSON: types.StringValue(`["not-an-object"]`),
	})
	if err == nil {
		t.Fatal("expected request_body_json object validation error")
	}
}

func TestBuildServiceRequestPayloadRejectsInvalidTopLevelResourcePoolParamsJSON(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequestPayload(context.Background(), ServiceRequestResourceModel{
		CatalogID:              types.StringValue("catalog-1"),
		BusinessGroupID:        types.StringValue("bg-1"),
		Name:                   types.StringValue("typed"),
		ResourcePoolParamsJSON: types.StringValue(`["not-an-object"]`),
	})
	if err == nil {
		t.Fatal("expected resource_pool_params_json object validation error")
	}
}

func TestBuildServiceRequestPayloadRejectsInvalidTypedSpecJSON(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequestPayload(context.Background(), ServiceRequestResourceModel{
		CatalogID:       types.StringValue("catalog-1"),
		BusinessGroupID: types.StringValue("bg-1"),
		Name:            types.StringValue("typed"),
		ResourceSpecs: mustResourceSpecsList(t, []serviceRequestResourceSpecModel{{
			Node:             types.StringValue("Compute"),
			ResourcePoolTags: mustStringListValue(t),
			SpecJSON:         types.StringValue(`["not-an-object"]`),
		}}),
	})
	if err == nil {
		t.Fatal("expected resource_specs[*].spec_json object validation error")
	}
}

func TestBuildServiceRequestPayloadRejectsConflictingResourceSpecs(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequestPayload(context.Background(), ServiceRequestResourceModel{
		CatalogID:       types.StringValue("catalog-1"),
		BusinessGroupID: types.StringValue("bg-1"),
		Name:            types.StringValue("typed"),
		RequestBodyJSON: types.StringValue(`{"resourceSpecs":[{"node":"Compute"}]}`),
		ResourceSpecs: mustResourceSpecsList(t, []serviceRequestResourceSpecModel{{
			Node:             types.StringValue("Compute"),
			ResourcePoolTags: mustStringListValue(t),
		}}),
	})
	if err == nil {
		t.Fatal("expected conflict error when resource_specs and request_body_json.resourceSpecs are both set")
	}
}

func TestBuildServiceRequestPayloadRequiresNodeForMultipleTypedResourceSpecs(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequestPayload(context.Background(), ServiceRequestResourceModel{
		CatalogID:       types.StringValue("catalog-1"),
		BusinessGroupID: types.StringValue("bg-1"),
		Name:            types.StringValue("typed"),
		ResourceSpecs: mustResourceSpecsList(t, []serviceRequestResourceSpecModel{
			{Node: types.StringValue("Compute"), ResourcePoolTags: mustStringListValue(t)},
			{ResourcePoolTags: mustStringListValue(t)},
		}),
	})
	if err == nil {
		t.Fatal("expected validation error for multiple typed resource_specs without node")
	}
}

func mustStringListValue(t *testing.T, values ...string) types.List {
	t.Helper()

	listValue, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("types.ListValueFrom returned diagnostics: %v", diags)
	}

	return listValue
}

func mustResourceSpecsList(t *testing.T, values []serviceRequestResourceSpecModel) types.List {
	t.Helper()

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"node":                      types.StringType,
		"type":                      types.StringType,
		"resource_pool_id":          types.StringType,
		"resource_pool_tags":        types.ListType{ElemType: types.StringType},
		"resource_pool_params_json": types.StringType,
		"spec_json":                 types.StringType,
	}}

	listValue, diags := listValueFromStructs(context.Background(), objectType, values)
	if diags.HasError() {
		t.Fatalf("listValueFromStructs returned diagnostics: %v", diags)
	}

	return listValue
}
