# Service Request Mapping Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor `smartcmp_service_request` so only stable request-shell and resource-default fields are typed, while service-specific resource parameters are mapped through JSON safely and predictably.

**Architecture:** Move service-request payload construction into a dedicated mapping helper so schema, merge order, validation, and tests are isolated from the resource CRUD flow. Treat request-level JSON and resource-spec-level JSON separately, with explicit conflict detection when typed `resource_specs` is used alongside `request_body_json.resourceSpecs`.

**Tech Stack:** Go, Terraform Plugin Framework, `terraform-plugin-framework-timeouts`, SmartCMP JSON APIs, Go test.

---

### Task 1: Add mapping-focused tests before code changes

**Files:**
- Create: `internal/provider/service_request_mapping_test.go`
- Modify: `internal/provider/resource_resource_test.go`

**Step 1: Write the failing unit tests for request-shell overrides**

Add tests that build a request model and assert:

- typed `catalog_id`, `business_group_id`, and `name` override conflicting JSON
- `project_id`, `request_user_id`, `count`, and `add_to_inventory` map to the correct SmartCMP keys
- unknown request-level JSON fields survive unchanged

Use a focused helper-style assertion similar to:

```go
payload, err := buildServiceRequestPayload(model)
if err != nil {
    t.Fatalf("build payload: %v", err)
}

if got := stringValue(payload["catalogId"]); got != "catalog-typed" {
    t.Fatalf("expected typed catalogId, got %q", got)
}
```

**Step 2: Run the new mapping test to verify it fails**

Run: `go test ./internal/provider -run TestBuildServiceRequestPayload -count=1 -v`

Expected: FAIL because the mapping helper does not exist yet.

**Step 3: Write the failing unit tests for typed `resource_specs` behavior**

Add tests that assert:

- `resource_specs[*].spec_json` becomes `resourceSpecs[*]`
- typed wrapper fields override keys from `spec_json`
- top-level resource defaults only fill missing values
- configuring typed `resource_specs` and `request_body_json.resourceSpecs` at the same time returns an error
- multiple typed `resource_specs` require `node`

**Step 4: Run the resource-spec tests to verify they fail**

Run: `go test ./internal/provider -run TestBuildServiceRequestResourceSpecs -count=1 -v`

Expected: FAIL because the new resource-spec mapping rules are not implemented.

**Step 5: Extend the existing resource create test to pin the final submit body**

In `internal/provider/resource_resource_test.go`, replace the old single-case JSON merge expectation with coverage for:

- top-level request overlays
- typed resource defaults
- final `resourceSpecs` array contents

**Step 6: Run the resource create test to verify it fails**

Run: `go test ./internal/provider -run TestServiceRequestResourceCreateAndDelete -count=1 -v`

Expected: FAIL because the existing submit-body builder still uses the old flat merge logic.

**Step 7: Commit the failing tests**

```bash
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 add \
  internal/provider/service_request_mapping_test.go \
  internal/provider/resource_resource_test.go
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 commit -m "test: define service request mapping behavior"
```

### Task 2: Implement the new request and resource-spec mapping helpers

**Files:**
- Create: `internal/provider/service_request_mapping.go`
- Modify: `internal/provider/resource_helpers.go`

**Step 1: Add typed models for the new schema pieces**

Create small helper structs for:

- request-shell inputs
- typed resource-spec wrapper inputs

The resource-spec wrapper should include:

```go
type serviceRequestResourceSpecModel struct {
    Node                   types.String `tfsdk:"node"`
    Type                   types.String `tfsdk:"type"`
    ResourcePoolID         types.String `tfsdk:"resource_pool_id"`
    ResourcePoolTags       types.List   `tfsdk:"resource_pool_tags"`
    ResourcePoolParamsJSON types.String `tfsdk:"resource_pool_params_json"`
    SpecJSON               types.String `tfsdk:"spec_json"`
}
```

**Step 2: Implement JSON parsing helpers for resource defaults**

Add helpers that:

- parse optional object JSON
- parse optional string-list values from Terraform lists
- detect whether a map already contains a value before filling defaults

Keep these helpers in `internal/provider/resource_helpers.go` if they are reused by tests and resource code.

**Step 3: Implement `buildServiceRequestPayload` in the new mapping file**

The function should:

- parse `request_body_json`
- reject non-object JSON
- overlay stable typed request fields
- delegate `resourceSpecs` construction
- preserve unknown fields

**Step 4: Implement `buildTypedResourceSpecs`**

For each typed item:

- parse `spec_json`
- overlay wrapper fields
- fill missing resource-pool fields from top-level defaults

Return a Go slice suitable for JSON encoding:

```go
[]map[string]any
```

**Step 5: Implement conflict detection**

If typed `resource_specs` exists and `request_body_json` already contains `resourceSpecs`, return a clear error like:

```go
errors.New("resource_specs cannot be used together with request_body_json.resourceSpecs")
```

**Step 6: Run the mapping-specific tests**

Run:

```bash
go test ./internal/provider -run 'TestBuildServiceRequestPayload|TestBuildServiceRequestResourceSpecs' -count=1 -v
```

Expected: PASS.

**Step 7: Commit the mapping helper implementation**

```bash
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 add \
  internal/provider/service_request_mapping.go \
  internal/provider/resource_helpers.go \
  internal/provider/service_request_mapping_test.go
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 commit -m "feat: add dynamic service request mapping helpers"
```

### Task 3: Refactor the resource schema and create flow to use the new mapping

**Files:**
- Modify: `internal/provider/stub_resources.go`
- Modify: `internal/provider/resource_resource_test.go`

**Step 1: Update `ServiceRequestResourceModel`**

Replace the old narrow shape with:

- stable request-shell fields
- top-level resource defaults
- typed `resource_specs`
- existing computed tracking fields

Specifically add:

- `ProjectID`
- `RequestUserID`
- `Count`
- `AddToInventory`
- `ResourcePoolTags`
- `ResourcePoolParamsJSON`
- `ResourceSpecs`

Keep `request_body_json` as the dynamic request-level container.

**Step 2: Update the Terraform schema**

In `Schema(...)`, add attributes and nested blocks for:

- `project_id`
- `request_user_id`
- `count`
- `add_to_inventory`
- `resource_pool_tags`
- `resource_pool_params_json`
- `resource_specs`

Use a nested attribute object for `resource_specs` with:

- `node`
- `type`
- `resource_pool_id`
- `resource_pool_tags`
- `resource_pool_params_json`
- `spec_json`

**Step 3: Replace the old payload merge in `Create`**

Swap this:

```go
payload := cloneMap(requestBody)
setString(payload, "catalogId", data.CatalogID)
```

for a single call:

```go
payload, err := buildServiceRequestPayload(ctx, data)
```

and return a diagnostic if mapping fails.

**Step 4: Keep CRUD semantics unchanged**

Do not change:

- request submit endpoint
- read/import behavior
- cancellation on delete
- terminal-state polling

Only the payload construction and schema should change in this task.

**Step 5: Run the focused resource tests**

Run:

```bash
go test ./internal/provider -run 'TestServiceRequestResourceCreateAndDelete|TestServiceRequestResourceCreateAndDeleteWithTypedSpecs' -count=1 -v
```

Expected: PASS.

**Step 6: Run the full provider package tests**

Run: `go test ./internal/provider -count=1`

Expected: PASS.

**Step 7: Commit the schema and resource refactor**

```bash
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 add \
  internal/provider/stub_resources.go \
  internal/provider/resource_resource_test.go
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 commit -m "feat: support typed service request shell and dynamic specs"
```

### Task 4: Update docs, examples, and acceptance coverage

**Files:**
- Modify: `README.md`
- Modify: `examples/README.md`
- Modify: `examples/resources/service-request/main.tf`
- Modify: `internal/provider/acceptance_test.go`

**Step 1: Update README resource documentation**

Document that:

- request-shell fields are fixed
- service-specific resource fields are dynamic
- `request_body_json` and `resource_specs[*].spec_json` are the extension points
- template and image fields are not universally required

**Step 2: Update the service-request example**

Replace any VM-fixed assumption with a typed-shell plus dynamic-spec example such as:

```hcl
resource "smartcmp_service_request" "linux_vm" {
  catalog_id        = "BUILD-IN-CATALOG-LINUX-VM"
  business_group_id = "bg-1"
  name              = "tf-linux-vm"
  resource_pool_id  = "rb-1"

  resource_specs = [{
    node      = "Compute"
    type      = "cloudchef.nodes.Compute"
    spec_json = jsonencode({
      computeProfileId   = "profile-1"
      logicTemplateId    = "template-1"
      credentialUser     = "root"
      credentialPassword = "example"
    })
  }]
}
```

**Step 3: Update acceptance tests to match the dynamic model**

Adjust acceptance coverage so Linux VM tests:

- verify provider login
- verify discovery chain
- verify profile lookup
- avoid asserting template/image requirements that are not universal

If a live service-request create test remains fixture-gated, document exactly why.

**Step 4: Run formatters and tests**

Run:

```bash
gofmt -w internal/provider/service_request_mapping.go internal/provider/service_request_mapping_test.go internal/provider/stub_resources.go internal/provider/resource_helpers.go internal/provider/resource_resource_test.go internal/provider/acceptance_test.go
terraform fmt -recursive /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1/examples
go test ./... -count=1
```

Expected: all tests PASS and examples are formatted.

**Step 5: Commit docs and acceptance updates**

```bash
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 add \
  README.md \
  examples/README.md \
  examples/resources/service-request/main.tf \
  internal/provider/acceptance_test.go
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 commit -m "docs: explain dynamic service request mapping"
```

### Task 5: Final verification and live check

**Files:**
- Modify if needed: `internal/provider/acceptance_test.go`
- Modify if needed: `README.md`

**Step 1: Re-run the full automated suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

**Step 2: Re-run environment-gated acceptance smoke tests**

Run:

```bash
SMARTCMP_TEST_BASE_URL="https://smartcmp.example.com" \
SMARTCMP_TEST_USERNAME="replace-with-username" \
SMARTCMP_TEST_PASSWORD="replace-with-password" \
SMARTCMP_TEST_TENANT_ID="default" \
SMARTCMP_TEST_INSECURE="true" \
NO_PROXY="smartcmp.example.com" \
go test ./internal/provider -run TestAcceptance -count=1 -v
```

Expected: PASS for smoke coverage, with fixture-gated cases skipped when the environment cannot satisfy them.

**Step 3: If live service request creation is attempted, capture the exact payload**

Compare the provider-generated payload against the observed SmartCMP UI payload for the target catalog. If the environment-specific payload differs, adjust only the example or fixture setup, not the generic mapping contract.

**Step 4: Commit final cleanup if any files changed**

```bash
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 add -A
git -C /Users/lfang/.config/superpowers/worktrees/smartcmp-terraform-provider/provider-v1 commit -m "test: verify dynamic service request mapping"
```
