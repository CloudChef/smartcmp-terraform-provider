package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

func TestServiceRequestResourceCreateAndDelete(t *testing.T) {
	t.Parallel()

	var submitBody map[string]any
	cancelCalled := false

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/generic-request/submit":
			_ = json.NewDecoder(r.Body).Decode(&submitBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, []map[string]any{{
				"id":              "req-1",
				"catalogId":       "catalog-1",
				"businessGroupId": "bg-1",
				"name":            "typed-name",
				"state":           "STARTED",
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/deployments":
			if got := r.URL.Query().Get("genericRequestId"); got != "req-1" {
				t.Fatalf("expected genericRequestId=req-1, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content":       []map[string]any{{"id": "dep-1"}},
				"totalElements": 1,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/generic-request/req-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "req-1",
				"state": "STARTED",
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/generic-request/req-1/cancel":
			cancelCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ServiceRequestResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"catalog_id":                tfStringValue("catalog-1"),
		"business_group_id":         tfStringValue("bg-1"),
		"name":                      tfStringValue("typed-name"),
		"description":               tfStringValue("typed-description"),
		"project_id":                tfStringValue("project-1"),
		"request_user_id":           tfStringValue("user-1"),
		"request_count":             tfInt64Value(2),
		"add_to_inventory":          tfBoolValue(true),
		"resource_pool_id":          tfStringValue("rb-typed"),
		"resource_pool_tags":        tfStringListValue("env:typed"),
		"resource_pool_params_json": tfStringValue(`{"available_zone_id":"zone-typed"}`),
		"request_body_json":         tfStringValue(`{"resourceBundleId":"rb-json","description":"json-description","exts":{"keep":"me"}}`),
		"resource_specs": tfObjectListValue(map[string]tftypes.Type{
			"node":                      tftypes.String,
			"type":                      tftypes.String,
			"resource_pool_id":          tftypes.String,
			"resource_pool_tags":        tftypes.List{ElementType: tftypes.String},
			"resource_pool_params_json": tftypes.String,
			"spec_json":                 tftypes.String,
		}, []map[string]tftypes.Value{{
			"node":                      tfStringValue("Compute"),
			"type":                      tfStringValue("cloudchef.nodes.Compute"),
			"resource_pool_id":          tfStringValue("rb-spec"),
			"resource_pool_tags":        tfStringListValue("env:spec"),
			"resource_pool_params_json": tfStringValue(`{"available_zone_id":"zone-spec"}`),
			"spec_json":                 tfStringValue(`{"computeProfileId":"cp-1","params":{"k":"v"}}`),
		}}),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(submitBody["catalogId"]); got != "catalog-1" {
		t.Fatalf("expected catalogId to be overlaid by typed field, got %q", got)
	}
	if got := stringValue(submitBody["resourceBundleId"]); got != "rb-typed" {
		t.Fatalf("expected resourceBundleId to be overlaid by typed field, got %q", got)
	}
	if got := stringValue(submitBody["description"]); got != "typed-description" {
		t.Fatalf("expected description to be overlaid by typed field, got %q", got)
	}
	if got := findStringPath(submitBody, "genericRequest", "description"); got != "typed-description" {
		t.Fatalf("expected genericRequest.description compatibility mapping, got %q", got)
	}
	if got := stringValue(submitBody["projectId"]); got != "project-1" {
		t.Fatalf("expected projectId to be set from project_id, got %q", got)
	}
	if got := stringValue(submitBody["groupId"]); got != "project-1" {
		t.Fatalf("expected groupId compatibility mapping, got %q", got)
	}
	if got := stringValue(submitBody["userId"]); got != "user-1" {
		t.Fatalf("expected userId to be set from request_user_id, got %q", got)
	}
	if got := stringValue(submitBody["requestUserId"]); got != "user-1" {
		t.Fatalf("expected requestUserId compatibility mapping, got %q", got)
	}
	if got := numberValue(submitBody["count"]); got != 2 {
		t.Fatalf("expected count 2, got %#v", submitBody["count"])
	}
	if got := boolValue(submitBody["addToInventory"]); !got {
		t.Fatalf("expected addToInventory true, got %#v", submitBody["addToInventory"])
	}
	if got := stringSliceFromAny(submitBody["resourceBundleTags"]); len(got) != 1 || got[0] != "env:typed" {
		t.Fatalf("expected top-level resourceBundleTags, got %#v", submitBody["resourceBundleTags"])
	}
	if got := asMap(submitBody["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-typed" {
		t.Fatalf("expected top-level resourceBundleParams, got %#v", submitBody["resourceBundleParams"])
	}
	rawSpecs, ok := submitBody["resourceSpecs"].([]any)
	if !ok || len(rawSpecs) != 1 {
		t.Fatalf("expected one resourceSpec, got %#v", submitBody["resourceSpecs"])
	}
	spec := asMap(rawSpecs[0])
	if got := stringValue(spec["resourceBundleId"]); got != "rb-spec" {
		t.Fatalf("expected typed resource spec override, got %q", got)
	}
	if got := stringSliceFromAny(spec["resourceBundleTags"]); len(got) != 1 || got[0] != "env:spec" {
		t.Fatalf("expected typed resource spec tags override, got %#v", spec["resourceBundleTags"])
	}
	if got := asMap(spec["resourceBundleParams"]); stringValue(got["available_zone_id"]) != "zone-spec" {
		t.Fatalf("expected typed resource spec params override, got %#v", spec["resourceBundleParams"])
	}
	if got := stringValue(spec["computeProfileId"]); got != "cp-1" {
		t.Fatalf("expected spec_json fields to survive, got %q", got)
	}
	if got := findStringPath(spec, "params", "k"); got != "v" {
		t.Fatalf("expected spec_json params to survive, got %q", got)
	}
	if got := findStringPath(submitBody, "exts", "keep"); got != "me" {
		t.Fatalf("expected unknown JSON fields to survive, got %q", got)
	}

	var createdState ServiceRequestResourceModel
	if diags := createResp.State.Get(context.Background(), &createdState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if createdState.ID.ValueString() != "req-1" {
		t.Fatalf("expected request id req-1, got %q", createdState.ID.ValueString())
	}
	var deploymentIDs []string
	if diags := createdState.DeploymentIDs.ElementsAs(context.Background(), &deploymentIDs, false); diags.HasError() {
		t.Fatalf("deployment ids decode diagnostics: %v", diags)
	}
	if len(deploymentIDs) != 1 || deploymentIDs[0] != "dep-1" {
		t.Fatalf("expected deployment_ids [dep-1], got %#v", deploymentIDs)
	}

	deleteReq := resource.DeleteRequest{
		State: createResp.State,
	}
	deleteResp := newResourceDeleteResponse(t, schema)
	resourceUnderTest.Delete(context.Background(), deleteReq, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", deleteResp.Diagnostics)
	}
	if !cancelCalled {
		t.Fatal("expected cancel endpoint to be called during delete")
	}
}

func TestServiceRequestResourceCreateRejectsConflictingResourceSpecs(t *testing.T) {
	t.Parallel()

	apiClient, err := client.New(client.Config{
		BaseURL:  "https://example.test",
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ServiceRequestResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"catalog_id":        tfStringValue("catalog-1"),
		"business_group_id": tfStringValue("bg-1"),
		"name":              tfStringValue("typed-name"),
		"request_body_json": tfStringValue(`{"resourceSpecs":[{"node":"Compute"}]}`),
		"resource_specs": tfObjectListValue(map[string]tftypes.Type{
			"node":                      tftypes.String,
			"type":                      tftypes.String,
			"resource_pool_id":          tftypes.String,
			"resource_pool_tags":        tftypes.List{ElementType: tftypes.String},
			"resource_pool_params_json": tftypes.String,
			"spec_json":                 tftypes.String,
		}, []map[string]tftypes.Value{{
			"node":               tfStringValue("Compute"),
			"resource_pool_tags": tfStringListValue(),
		}}),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("expected create diagnostics for conflicting resourceSpecs inputs")
	}
}

func TestResourceOperationResourceCreateDeployment(t *testing.T) {
	t.Parallel()

	var deploymentBody map[string]any

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/deployments/dep-1/day2-op":
			_ = json.NewDecoder(r.Body).Decode(&deploymentBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":            "task-1",
				"state":         "CREATED",
				"deploymentId":  "dep-1",
				"operationName": "powerOn",
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ResourceOperationResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"target_kind":             tfStringValue("deployment"),
		"target_id":               tfStringValue("dep-1"),
		"operation":               tfStringValue("powerOn"),
		"comment":                 tfStringValue("from-test"),
		"scheduled_time":          tfStringValue("2026-03-22T10:00:00Z"),
		"parameters_json":         tfStringValue(`{"answer":42}`),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(deploymentBody["operationName"]); got != "powerOn" {
		t.Fatalf("expected operationName powerOn, got %q", got)
	}
	if got := stringValue(deploymentBody["comment"]); got != "from-test" {
		t.Fatalf("expected comment from-test, got %q", got)
	}
	if got := findStringPath(deploymentBody, "scheduledTaskMetadataRequest", "scheduledTime"); got != "2026-03-22T10:00:00Z" {
		t.Fatalf("expected scheduled time to be set, got %q", got)
	}
	params, ok := deploymentBody["params"].(map[string]any)
	if !ok || numberValue(params["answer"]) != 42 {
		t.Fatalf("expected parameters_json to be wrapped into params, got %#v", deploymentBody["params"])
	}

	var createdState ResourceOperationResourceModel
	if diags := createResp.State.Get(context.Background(), &createdState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if createdState.TaskID.ValueString() != "task-1" || createdState.DeploymentID.ValueString() != "dep-1" {
		t.Fatalf("unexpected created state: %+v", createdState)
	}
}

func TestResourceOperationResourceCreateDeploymentPassesThroughFullPayload(t *testing.T) {
	t.Parallel()

	var deploymentBody map[string]any

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/deployments/dep-2/day2-op":
			_ = json.NewDecoder(r.Body).Decode(&deploymentBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":            "task-3",
				"state":         "CREATED",
				"deploymentId":  "dep-2",
				"operationName": "restart",
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ResourceOperationResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"target_kind":             tfStringValue("deployment"),
		"target_id":               tfStringValue("dep-2"),
		"operation":               tfStringValue("restart"),
		"parameters_json":         tfStringValue(`{"operationParamJson":"{\"mode\":\"safe\"}","params":{"reason":"terraform"}}`),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(deploymentBody["operationName"]); got != "restart" {
		t.Fatalf("expected operationName restart, got %q", got)
	}
	if got := stringValue(deploymentBody["operationParamJson"]); got != `{"mode":"safe"}` {
		t.Fatalf("expected operationParamJson to be preserved, got %q", got)
	}
	params, ok := deploymentBody["params"].(map[string]any)
	if !ok || stringValue(params["reason"]) != "terraform" {
		t.Fatalf("expected full params payload to be preserved, got %#v", deploymentBody["params"])
	}
}

func TestResourceOperationResourceCreateAndDeleteResourceTask(t *testing.T) {
	t.Parallel()

	var resourceBody map[string]any
	cancelCalled := false

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			_ = json.NewDecoder(r.Body).Decode(&resourceBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"results": map[string]any{
					"res-1": map[string]any{
						"id":               "task-2",
						"state":            "STARTED",
						"resourceIds":      []string{"res-1"},
						"genericRequestId": "req-2",
					},
				},
				"webOperation": false,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":          "task-2",
				"state":       "STARTED",
				"resourceIds": []string{"res-1"},
			}))
		case r.Method == http.MethodPut && r.URL.Path == "/platform-api/tasks/task-2/cancel":
			cancelCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-2",
				"state": "CANCELLED",
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ResourceOperationResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"target_kind":             tfStringValue("resource"),
		"target_id":               tfStringValue("res-1"),
		"operation":               tfStringValue("powerOn"),
		"parameters_json":         tfStringValue(`{"reason":"terraform"}`),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(resourceBody["operationId"]); got != "powerOn" {
		t.Fatalf("expected operationId powerOn, got %q", got)
	}
	if got := stringSliceFromAny(resourceBody["resourceIds"]); len(got) != 1 || got[0] != "res-1" {
		t.Fatalf("expected resourceIds [res-1], got %#v", resourceBody["resourceIds"])
	}
	params, ok := resourceBody["executeParameters"].(map[string]any)
	if !ok || stringValue(params["reason"]) != "terraform" {
		t.Fatalf("expected executeParameters to contain reason, got %#v", resourceBody["executeParameters"])
	}

	deleteReq := resource.DeleteRequest{
		State: createResp.State,
	}
	deleteResp := newResourceDeleteResponse(t, schema)
	resourceUnderTest.Delete(context.Background(), deleteReq, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", deleteResp.Diagnostics)
	}
	if !cancelCalled {
		t.Fatal("expected task cancel endpoint to be called during delete")
	}
}

func TestResourceOperationResourceDeleteCancelsSubmittedTask(t *testing.T) {
	t.Parallel()

	cancelCalled := false

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-submitted":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-submitted",
				"state": "SUBMITTED",
			}))
		case r.Method == http.MethodPut && r.URL.Path == "/platform-api/tasks/task-submitted/cancel":
			cancelCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-submitted",
				"state": "CANCELLED",
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ResourceOperationResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	deleteReq := newResourceDeleteRequest(t, schema, map[string]tftypes.Value{
		"id":                      tfStringValue("task-submitted"),
		"target_kind":             tfStringValue("resource"),
		"target_id":               tfStringValue("res-1"),
		"operation":               tfStringValue("powerOn"),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	deleteResp := newResourceDeleteResponse(t, schema)
	resourceUnderTest.Delete(context.Background(), deleteReq, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", deleteResp.Diagnostics)
	}
	if !cancelCalled {
		t.Fatal("expected cancel endpoint to be called for submitted task")
	}
}

func TestResourceOperationResourceCreatePassesThroughFullPayload(t *testing.T) {
	t.Parallel()

	var resourceBody map[string]any

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			_ = json.NewDecoder(r.Body).Decode(&resourceBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"results": map[string]any{
					"res-2": map[string]any{
						"id":          "task-4",
						"state":       "STARTED",
						"resourceIds": []string{"res-2"},
					},
				},
				"webOperation": false,
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer server.Close()

	apiClient, err := client.New(client.Config{
		BaseURL:  server.URL,
		Username: "user",
		Password: "password",
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	resourceUnderTest := &ResourceOperationResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"target_kind":             tfStringValue("resource"),
		"target_id":               tfStringValue("res-2"),
		"operation":               tfStringValue("resize"),
		"parameters_json":         tfStringValue(`{"executeParameters":{"computeProfileId":"profile-large"},"day2InventoryRequest":{"mode":"keep"}}`),
		"wait_for_terminal_state": tfBoolValue(false),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(resourceBody["operationId"]); got != "resize" {
		t.Fatalf("expected operationId resize, got %q", got)
	}
	if got := stringSliceFromAny(resourceBody["resourceIds"]); len(got) != 1 || got[0] != "res-2" {
		t.Fatalf("expected resourceIds [res-2], got %#v", resourceBody["resourceIds"])
	}
	params, ok := resourceBody["executeParameters"].(map[string]any)
	if !ok || stringValue(params["computeProfileId"]) != "profile-large" {
		t.Fatalf("expected executeParameters to be preserved, got %#v", resourceBody["executeParameters"])
	}
	if got := findStringPath(resourceBody, "day2InventoryRequest", "mode"); got != "keep" {
		t.Fatalf("expected day2InventoryRequest to be preserved, got %q", got)
	}
}
