package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

func TestVirtualMachineResourceCreate(t *testing.T) {
	t.Parallel()

	var submitBody map[string]any

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/generic-request/submit":
			_ = json.NewDecoder(r.Body).Decode(&submitBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, []map[string]any{{
				"id":    "req-vm-1",
				"state": "INITIALING",
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/generic-request/req-vm-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "req-vm-1",
				"state": "FINISHED",
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/deployments":
			if got := r.URL.Query().Get("genericRequestId"); got != "req-vm-1" {
				t.Fatalf("expected genericRequestId=req-vm-1, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{{"id": "dep-vm-1"}},
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/all-status":
			if got := r.URL.Query().Get("deploymentId"); got != "dep-vm-1" {
				t.Fatalf("expected deploymentId=dep-vm-1, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{{
					"id":           "res-vm-1",
					"deploymentId": "dep-vm-1",
					"status":       "started",
					"instanceType": "ecs.t6-c1m1.large",
					"cpu":          2,
					"memoryInGB":   2.0,
					"resourceType": "cloudchef.aliyun.nodes.Instance",
				}},
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"catalog_id":        tfStringValue("catalog-1"),
		"business_group_id": tfStringValue("bg-1"),
		"name":              tfStringValue("vm-demo"),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if got := stringValue(submitBody["catalogId"]); got != "catalog-1" {
		t.Fatalf("expected catalogId catalog-1, got %q", got)
	}
	if got := stringValue(submitBody["businessGroupId"]); got != "bg-1" {
		t.Fatalf("expected businessGroupId bg-1, got %q", got)
	}
	if got := stringValue(submitBody["name"]); got != "vm-demo" {
		t.Fatalf("expected name vm-demo, got %q", got)
	}
	if _, ok := submitBody["resourceBundleId"]; ok {
		t.Fatalf("expected resourceBundleId to be omitted when unset, got %#v", submitBody["resourceBundleId"])
	}
	rawSpecs := extractItems(submitBody["resourceSpecs"])
	if len(rawSpecs) != 1 {
		t.Fatalf("expected one resource spec, got %#v", submitBody["resourceSpecs"])
	}
	spec := rawSpecs[0]
	if got := stringValue(spec["node"]); got != "Compute" {
		t.Fatalf("expected node Compute, got %q", got)
	}
	if got := stringValue(spec["type"]); got != "cloudchef.nodes.Compute" {
		t.Fatalf("expected type cloudchef.nodes.Compute, got %q", got)
	}
	if _, ok := spec["credentialPassword"]; ok {
		t.Fatalf("expected unset credentialPassword to be omitted")
	}

	var createdState VirtualMachineResourceModel
	if diags := createResp.State.Get(context.Background(), &createdState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if createdState.ID.ValueString() != "res-vm-1" {
		t.Fatalf("expected id res-vm-1, got %q", createdState.ID.ValueString())
	}
	if createdState.RequestID.ValueString() != "req-vm-1" {
		t.Fatalf("expected request_id req-vm-1, got %q", createdState.RequestID.ValueString())
	}
	if createdState.DeploymentID.ValueString() != "dep-vm-1" {
		t.Fatalf("expected deployment_id dep-vm-1, got %q", createdState.DeploymentID.ValueString())
	}
	if createdState.InstanceTypeActual.ValueString() != "ecs.t6-c1m1.large" {
		t.Fatalf("expected instance_type_actual ecs.t6-c1m1.large, got %q", createdState.InstanceTypeActual.ValueString())
	}
	if createdState.PowerState.ValueString() != "started" {
		t.Fatalf("expected power_state started, got %q", createdState.PowerState.ValueString())
	}
}

func TestBuildVirtualMachineRequestPayloadAllowsSystemDiskWithoutName(t *testing.T) {
	t.Parallel()

	data := VirtualMachineResourceModel{
		CatalogID:       types.StringValue("catalog-1"),
		BusinessGroupID: types.StringValue("bg-1"),
		Name:            types.StringValue("vm-demo"),
		SystemDisk: types.ObjectValueMust(
			map[string]attr.Type{
				"size":           types.Int64Type,
				"is_system_disk": types.BoolType,
				"volume_type":    types.StringType,
				"disk_policy":    types.StringType,
				"disk_tags":      types.ListType{ElemType: types.StringType},
			},
			map[string]attr.Value{
				"size":           types.Int64Value(40),
				"is_system_disk": types.BoolValue(true),
				"volume_type":    types.StringNull(),
				"disk_policy":    types.StringNull(),
				"disk_tags":      types.ListNull(types.StringType),
			},
		),
	}

	payload, err := buildVirtualMachineRequestPayload(context.Background(), &data)
	if err != nil {
		t.Fatalf("buildVirtualMachineRequestPayload returned error: %v", err)
	}

	specs, ok := payload["resourceSpecs"].([]map[string]any)
	if !ok {
		t.Fatalf("expected resourceSpecs to be []map[string]any, got %#v", payload["resourceSpecs"])
	}
	if len(specs) != 1 {
		t.Fatalf("expected one resource spec, got %#v", payload["resourceSpecs"])
	}

	systemDisk := asMap(specs[0]["systemDisk"])
	if got := numberValue(systemDisk["size"]); got != 40 {
		t.Fatalf("expected system disk size 40, got %#v", systemDisk["size"])
	}
	if got := systemDisk["is_system_disk"]; got != true {
		t.Fatalf("expected is_system_disk true, got %#v", systemDisk["is_system_disk"])
	}
	if _, ok := systemDisk["name"]; ok {
		t.Fatalf("expected system disk name to be omitted, got %#v", systemDisk["name"])
	}
}

func TestVirtualMachineResourceCreateFallsBackToNodeLookupWhenDeploymentSearchIsEmpty(t *testing.T) {
	t.Parallel()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/generic-request/submit":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, []map[string]any{{
				"id":          "req-vm-fallback",
				"state":       "INITIALING",
				"createdDate": 1000,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/generic-request/req-vm-fallback":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":          "req-vm-fallback",
				"state":       "FINISHED",
				"createdDate": 1000,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/deployments":
			if got := r.URL.Query().Get("genericRequestId"); got != "req-vm-fallback" {
				t.Fatalf("expected genericRequestId=req-vm-fallback, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{"content": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/all-status":
			if got := r.URL.Query().Get("deploymentId"); got != "" {
				t.Fatalf("expected deploymentId to be empty for fallback lookup, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{
					{
						"id":           "old-res",
						"deploymentId": "old-dep",
						"createdDate":  900,
						"name":         "vm-demo",
						"status":       "started",
						"instanceType": "ecs.t6-c1m1.large",
						"resourceType": "cloudchef.aliyun.nodes.Instance",
					},
					{
						"id":           "res-vm-fallback",
						"deploymentId": "dep-vm-fallback",
						"createdDate":  1200,
						"name":         "vm-demo",
						"status":       "started",
						"instanceType": "ecs.t6-c1m2.large",
						"cpu":          2,
						"memoryInGB":   4.0,
						"resourceType": "cloudchef.aliyun.nodes.Instance",
					},
				},
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"catalog_id":        tfStringValue("catalog-1"),
		"business_group_id": tfStringValue("bg-1"),
		"name":              tfStringValue("vm-demo"),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	var createdState VirtualMachineResourceModel
	if diags := createResp.State.Get(context.Background(), &createdState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if createdState.ID.ValueString() != "res-vm-fallback" {
		t.Fatalf("expected id res-vm-fallback, got %q", createdState.ID.ValueString())
	}
	if createdState.DeploymentID.ValueString() != "dep-vm-fallback" {
		t.Fatalf("expected deployment_id dep-vm-fallback, got %q", createdState.DeploymentID.ValueString())
	}
}

func TestVirtualMachineResourceCreateReconcilesConfiguredPowerState(t *testing.T) {
	t.Parallel()

	currentStatus := "started"
	var operations []map[string]any

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/generic-request/submit":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, []map[string]any{{
				"id":          "req-vm-stop",
				"state":       "INITIALING",
				"createdDate": 1000,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/generic-request/req-vm-stop":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":          "req-vm-stop",
				"state":       "FINISHED",
				"createdDate": 1000,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/deployments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{{"id": "dep-vm-stop"}},
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/all-status":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{{
					"id":           "res-vm-stop",
					"deploymentId": "dep-vm-stop",
					"status":       currentStatus,
					"instanceType": "ecs.t6-c1m1.large",
					"resourceType": "cloudchef.aliyun.nodes.Instance",
				}},
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			operations = append(operations, body)
			currentStatus = "stopped"
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"results": map[string]any{
					"res-vm-stop": map[string]any{
						"id":          "task-stop-create",
						"state":       "STARTED",
						"resourceIds": []string{"res-vm-stop"},
					},
				},
				"webOperation": false,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-stop-create":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-stop-create", "state": "FINISHED"}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-stop":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-stop",
				"deploymentId": "dep-vm-stop",
				"status":       currentStatus,
				"instanceType": "ecs.t6-c1m1.large",
				"resourceType": "cloudchef.aliyun.nodes.Instance",
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	createReq := newResourceCreateRequest(t, schema, map[string]tftypes.Value{
		"catalog_id":        tfStringValue("catalog-1"),
		"business_group_id": tfStringValue("bg-1"),
		"name":              tfStringValue("vm-demo"),
		"power_state":       tfStringValue("stopped"),
	})
	createResp := newResourceCreateResponse(t, schema)
	resourceUnderTest.Create(context.Background(), createReq, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	if len(operations) != 1 {
		t.Fatalf("expected 1 power operation, got %d", len(operations))
	}
	if got := stringValue(operations[0]["operationId"]); got != "stop" {
		t.Fatalf("expected stop operation, got %q", got)
	}

	var createdState VirtualMachineResourceModel
	if diags := createResp.State.Get(context.Background(), &createdState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if createdState.PowerState.ValueString() != "stopped" {
		t.Fatalf("expected power_state stopped, got %q", createdState.PowerState.ValueString())
	}
	if createdState.Status.ValueString() != "stopped" {
		t.Fatalf("expected status stopped, got %q", createdState.Status.ValueString())
	}
}

func TestVirtualMachineResourceUpdateResizesAndStarts(t *testing.T) {
	t.Parallel()

	var operations []map[string]any
	currentStatus := "started"
	currentType := "ecs.t6-c1m1.large"
	currentCPU := 2
	currentMemory := 2.0

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-1",
				"deploymentId": "dep-vm-1",
				"status":       currentStatus,
				"instanceType": currentType,
				"cpu":          currentCPU,
				"memoryInGB":   currentMemory,
				"resourceType": "cloudchef.aliyun.nodes.Instance",
				"cloudEntryId": "ce-1",
				"regionId":     "cn-shanghai",
				"zoneId":       "cn-shanghai-e",
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			operations = append(operations, body)
			op := stringValue(body["operationId"])
			switch op {
			case "stop":
				currentStatus = "stopped"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results": map[string]any{
						"res-vm-1": map[string]any{
							"id":          "task-stop-1",
							"state":       "STARTED",
							"resourceIds": []string{"res-vm-1"},
						},
					},
					"webOperation": false,
				}))
			case "resize":
				currentType = "ecs.t6-c1m2.large"
				currentCPU = 2
				currentMemory = 4.0
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results": map[string]any{
						"res-vm-1": map[string]any{
							"id":          "task-resize-1",
							"state":       "STARTED",
							"resourceIds": []string{"res-vm-1"},
						},
					},
					"webOperation": false,
				}))
			case "start":
				currentStatus = "started"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results": map[string]any{
						"res-vm-1": map[string]any{
							"id":          "task-start-1",
							"state":       "STARTED",
							"resourceIds": []string{"res-vm-1"},
						},
					},
					"webOperation": false,
				}))
			default:
				t.Fatalf("unexpected operation %q", op)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-stop-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-stop-1",
				"state": "FINISHED",
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-resize-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-resize-1",
				"state": "FINISHED",
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-start-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":    "task-start-1",
				"state": "FINISHED",
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	updateReq := newResourceUpdateRequest(t, schema,
		map[string]tftypes.Value{
			"id":                   tfStringValue("res-vm-1"),
			"resource_id":          tfStringValue("res-vm-1"),
			"deployment_id":        tfStringValue("dep-vm-1"),
			"request_id":           tfStringValue("req-vm-1"),
			"catalog_id":           tfStringValue("catalog-1"),
			"business_group_id":    tfStringValue("bg-1"),
			"name":                 tfStringValue("vm-demo"),
			"instance_type":        tfStringValue("ecs.t6-c1m1.large"),
			"instance_type_actual": tfStringValue("ecs.t6-c1m1.large"),
			"status":               tfStringValue("started"),
			"cpu":                  tfInt64Value(2),
			"memory_gb":            tfFloat64Value(2.0),
			"start_after_resize":   tfBoolValue(false),
		},
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"request_id":         tfStringValue("req-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m2.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(4.0),
			"start_after_resize": tfBoolValue(true),
		},
	)
	updateResp := newResourceUpdateResponse(t, schema)
	resourceUnderTest.Update(context.Background(), updateReq, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}

	if len(operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(operations))
	}
	if got := stringValue(operations[0]["operationId"]); got != "stop" {
		t.Fatalf("expected first operation stop, got %q", got)
	}
	if got := stringValue(operations[1]["operationId"]); got != "resize" {
		t.Fatalf("expected second operation resize, got %q", got)
	}
	if got := stringValue(operations[2]["operationId"]); got != "start" {
		t.Fatalf("expected third operation start, got %q", got)
	}
	params := asMap(operations[1]["executeParameters"])
	if got := stringValue(params["flavorId"]); got != "ecs.t6-c1m2.large" {
		t.Fatalf("expected resize flavorId ecs.t6-c1m2.large, got %q", got)
	}
	if got := numberValue(params["cpu"]); got != 2 {
		t.Fatalf("expected resize cpu 2, got %#v", params["cpu"])
	}
	if got := numberValue(params["memory"]); got != 4 {
		t.Fatalf("expected resize memory 4, got %#v", params["memory"])
	}

	var updatedState VirtualMachineResourceModel
	if diags := updateResp.State.Get(context.Background(), &updatedState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if updatedState.InstanceTypeActual.ValueString() != "ecs.t6-c1m2.large" {
		t.Fatalf("expected instance_type_actual ecs.t6-c1m2.large, got %q", updatedState.InstanceTypeActual.ValueString())
	}
	if updatedState.Status.ValueString() != "started" {
		t.Fatalf("expected status started, got %q", updatedState.Status.ValueString())
	}
}

func TestVirtualMachineResourceUpdateResizeWithoutStart(t *testing.T) {
	t.Parallel()

	var operations []map[string]any
	currentStatus := "started"
	currentType := "ecs.t6-c1m1.large"

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-1",
				"deploymentId": "dep-vm-1",
				"status":       currentStatus,
				"instanceType": currentType,
				"cpu":          2,
				"memoryInGB":   2.0,
				"resourceType": "cloudchef.aliyun.nodes.Instance",
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			operations = append(operations, body)
			switch stringValue(body["operationId"]) {
			case "stop":
				currentStatus = "stopped"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results":      map[string]any{"res-vm-1": map[string]any{"id": "task-stop-2", "state": "STARTED", "resourceIds": []string{"res-vm-1"}}},
					"webOperation": false,
				}))
			case "resize":
				currentType = "ecs.t6-c1m2.large"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results":      map[string]any{"res-vm-1": map[string]any{"id": "task-resize-2", "state": "STARTED", "resourceIds": []string{"res-vm-1"}}},
					"webOperation": false,
				}))
			default:
				t.Fatalf("unexpected operation %q", stringValue(body["operationId"]))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-stop-2":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-stop-2", "state": "FINISHED"}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-resize-2":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-resize-2", "state": "FINISHED"}))
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	updateReq := newResourceUpdateRequest(t, schema,
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m1.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(2.0),
			"start_after_resize": tfBoolValue(false),
		},
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m2.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(4.0),
			"start_after_resize": tfBoolValue(false),
		},
	)
	updateResp := newResourceUpdateResponse(t, schema)
	resourceUnderTest.Update(context.Background(), updateReq, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}

	if len(operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(operations))
	}
	if got := stringValue(operations[0]["operationId"]); got != "stop" {
		t.Fatalf("expected first operation stop, got %q", got)
	}
	if got := stringValue(operations[1]["operationId"]); got != "resize" {
		t.Fatalf("expected second operation resize, got %q", got)
	}
}

func TestVirtualMachineResourceUpdatePowerStateOnly(t *testing.T) {
	t.Parallel()

	var operations []map[string]any
	currentStatus := "started"

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-1",
				"deploymentId": "dep-vm-1",
				"status":       currentStatus,
				"instanceType": "ecs.t6-c1m1.large",
				"cpu":          2,
				"memoryInGB":   2.0,
				"resourceType": "cloudchef.aliyun.nodes.Instance",
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			operations = append(operations, body)
			currentStatus = "stopped"
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"results": map[string]any{
					"res-vm-1": map[string]any{
						"id":          "task-stop-only",
						"state":       "STARTED",
						"resourceIds": []string{"res-vm-1"},
					},
				},
				"webOperation": false,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-stop-only":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-stop-only", "state": "FINISHED"}))
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	updateReq := newResourceUpdateRequest(t, schema,
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m1.large"),
			"power_state":        tfStringValue("started"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(2.0),
			"start_after_resize": tfBoolValue(false),
		},
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m1.large"),
			"power_state":        tfStringValue("stopped"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(2.0),
			"start_after_resize": tfBoolValue(false),
		},
	)
	updateResp := newResourceUpdateResponse(t, schema)
	resourceUnderTest.Update(context.Background(), updateReq, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}

	if len(operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(operations))
	}
	if got := stringValue(operations[0]["operationId"]); got != "stop" {
		t.Fatalf("expected stop operation, got %q", got)
	}

	var updatedState VirtualMachineResourceModel
	if diags := updateResp.State.Get(context.Background(), &updatedState); diags.HasError() {
		t.Fatalf("state decode diagnostics: %v", diags)
	}
	if updatedState.PowerState.ValueString() != "stopped" {
		t.Fatalf("expected power_state stopped, got %q", updatedState.PowerState.ValueString())
	}
	if updatedState.Status.ValueString() != "stopped" {
		t.Fatalf("expected status stopped, got %q", updatedState.Status.ValueString())
	}
}

func TestVirtualMachineResourceUpdateResizeFailureSurfacesTaskMessage(t *testing.T) {
	t.Parallel()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-1",
				"deploymentId": "dep-vm-1",
				"status":       "stopped",
				"instanceType": "ecs.t6-c1m1.large",
				"cpu":          2,
				"memoryInGB":   2.0,
				"resourceType": "cloudchef.aliyun.nodes.Instance",
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if got := stringValue(body["operationId"]); got != "resize" {
				t.Fatalf("expected resize operation, got %q", got)
			}
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"results": map[string]any{
					"res-vm-1": map[string]any{
						"id":          "task-resize-failed",
						"state":       "STARTED",
						"resourceIds": []string{"res-vm-1"},
					},
				},
				"webOperation": false,
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-resize-failed":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":        "task-resize-failed",
				"state":     "FAILED",
				"resultMsg": "backend rejected resize",
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	updateReq := newResourceUpdateRequest(t, schema,
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m1.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(2.0),
			"start_after_resize": tfBoolValue(false),
		},
		map[string]tftypes.Value{
			"id":                 tfStringValue("res-vm-1"),
			"resource_id":        tfStringValue("res-vm-1"),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m2.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(4.0),
			"start_after_resize": tfBoolValue(false),
		},
	)
	updateResp := newResourceUpdateResponse(t, schema)
	resourceUnderTest.Update(context.Background(), updateReq, &updateResp)
	if !updateResp.Diagnostics.HasError() {
		t.Fatalf("expected resize failure diagnostics")
	}
	if got := updateResp.Diagnostics.Errors()[0].Detail(); got != `resource operation "resize" finished in state FAILED: backend rejected resize` {
		t.Fatalf("unexpected diagnostic detail: %q", got)
	}
}

func TestVirtualMachineResourceUpdateResolvesNodeByDeployment(t *testing.T) {
	t.Parallel()

	var operations []map[string]any
	currentStatus := "started"
	currentType := "ecs.t6-c1m1.large"

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/res-vm-1":
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"id":           "res-vm-1",
				"deploymentId": "dep-vm-1",
				"status":       currentStatus,
				"instanceType": currentType,
				"cpu":          2,
				"memoryInGB":   4.0,
				"resourceType": "cloudchef.aliyun.nodes.Instance",
			}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/all-status":
			if got := r.URL.Query().Get("deploymentId"); got != "dep-vm-1" {
				t.Fatalf("expected deploymentId dep-vm-1, got %q", got)
			}
			_, _ = w.Write(encodeJSON(t, map[string]any{
				"content": []map[string]any{{
					"id":           "res-vm-1",
					"deploymentId": "dep-vm-1",
					"status":       currentStatus,
					"instanceType": currentType,
					"cpu":          2,
					"memoryInGB":   2.0,
					"resourceType": "cloudchef.aliyun.nodes.Instance",
				}},
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/platform-api/nodes/resource-operations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			operations = append(operations, body)
			switch stringValue(body["operationId"]) {
			case "stop":
				currentStatus = "stopped"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results":      map[string]any{"res-vm-1": map[string]any{"id": "task-stop-3", "state": "STARTED", "resourceIds": []string{"res-vm-1"}}},
					"webOperation": false,
				}))
			case "resize":
				currentType = "ecs.t6-c1m2.large"
				_, _ = w.Write(encodeJSON(t, map[string]any{
					"results":      map[string]any{"res-vm-1": map[string]any{"id": "task-resize-3", "state": "STARTED", "resourceIds": []string{"res-vm-1"}}},
					"webOperation": false,
				}))
			default:
				t.Fatalf("unexpected operation %q", stringValue(body["operationId"]))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-stop-3":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-stop-3", "state": "FINISHED"}))
		case r.Method == http.MethodGet && r.URL.Path == "/platform-api/tasks/task-resize-3":
			_, _ = w.Write(encodeJSON(t, map[string]any{"id": "task-resize-3", "state": "FINISHED"}))
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

	resourceUnderTest := &VirtualMachineResource{client: apiClient}
	schema := requireResourceSchema(t, resourceUnderTest)

	updateReq := newResourceUpdateRequest(t, schema,
		map[string]tftypes.Value{
			"id":                 tfStringValue(""),
			"resource_id":        tfStringValue(""),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m1.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(2.0),
			"start_after_resize": tfBoolValue(false),
		},
		map[string]tftypes.Value{
			"id":                 tfStringValue(""),
			"resource_id":        tfStringValue(""),
			"deployment_id":      tfStringValue("dep-vm-1"),
			"catalog_id":         tfStringValue("catalog-1"),
			"business_group_id":  tfStringValue("bg-1"),
			"name":               tfStringValue("vm-demo"),
			"instance_type":      tfStringValue("ecs.t6-c1m2.large"),
			"cpu":                tfInt64Value(2),
			"memory_gb":          tfFloat64Value(4.0),
			"start_after_resize": tfBoolValue(false),
		},
	)
	updateResp := newResourceUpdateResponse(t, schema)
	resourceUnderTest.Update(context.Background(), updateReq, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}

	if len(operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(operations))
	}
}

func TestWaitForVirtualMachineNodeTimeout(t *testing.T) {
	t.Parallel()

	server := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/platform-api/nodes/all-status" {
			_, _ = w.Write(encodeJSON(t, map[string]any{"content": []map[string]any{}}))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if _, err := waitForVirtualMachineNode(ctx, apiClient, "dep-vm-1"); err == nil {
		t.Fatalf("expected timeout error when no VM node appears")
	}
}
