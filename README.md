# SmartCMP Terraform Provider

The SmartCMP Terraform provider exposes CloudChef SmartCMP catalogs, discovery data, profile sizing, and request-oriented resources through Terraform.

## Requirements

- Terraform `>= 1.0`
- Go `>= 1.25` for local development
- Access to a SmartCMP platform endpoint

## Authentication

The provider supports both private SmartCMP deployments and the public SmartCMP SaaS console.

Passwords are accepted as plaintext and are MD5-hashed by the provider client before login to match SmartCMP client behavior. For open source usage, prefer environment variables or sensitive Terraform variables instead of committing credentials to HCL.

Export credentials locally:

```bash
export SMARTCMP_BASE_URL="https://smartcmp.example.com"
export SMARTCMP_USERNAME="replace-with-username"
export SMARTCMP_PASSWORD="replace-with-password"
export SMARTCMP_TENANT_ID="default"
export SMARTCMP_INSECURE="true"
```

Then keep the provider block minimal:

```hcl
terraform {
  required_providers {
    smartcmp = {
      source  = "cloudchef/smartcmp"
      version = "~> 0.1"
    }
  }
}

provider "smartcmp" {
  # Read endpoint and credentials from SMARTCMP_* environment variables.
}
```

When a private deployment is hosted on a SmartCMP-owned domain, force private auth explicitly:

```hcl
provider "smartcmp" {
  auth_mode = "private"
}
```

The provider also reads these environment variables:

- `SMARTCMP_BASE_URL`
- `SMARTCMP_USERNAME`
- `SMARTCMP_PASSWORD`
- `SMARTCMP_TENANT_ID`
- `SMARTCMP_AUTH_MODE`
- `SMARTCMP_INSECURE`

## Multi-Tenant Providers

Use provider aliases when a configuration needs to talk to more than one SmartCMP tenant or endpoint.

```hcl
variable "prod_base_url" {
  type = string
}

variable "prod_username" {
  type = string
}

variable "prod_password" {
  type      = string
  sensitive = true
}

variable "prod_tenant_id" {
  type = string
}

variable "dev_base_url" {
  type = string
}

variable "dev_username" {
  type = string
}

variable "dev_password" {
  type      = string
  sensitive = true
}

variable "dev_tenant_id" {
  type = string
}

provider "smartcmp" {
  alias     = "prod"
  base_url  = var.prod_base_url
  username  = var.prod_username
  password  = var.prod_password
  tenant_id = var.prod_tenant_id
}

provider "smartcmp" {
  alias     = "dev"
  base_url  = var.dev_base_url
  username  = var.dev_username
  password  = var.dev_password
  tenant_id = var.dev_tenant_id
}
```

When a resource or data source should run in a specific tenant, bind it with `provider = smartcmp.dev`.

## Data Sources

The provider includes the following data sources:

- `smartcmp_published_catalogs`
- `smartcmp_catalog_business_groups`
- `smartcmp_catalog_component`
- `smartcmp_resource_pools`
- `smartcmp_applications`
- `smartcmp_os_templates`
- `smartcmp_cloud_entry_types`
- `smartcmp_images`
- `smartcmp_profiles`
- `smartcmp_deployment_actions`
- `smartcmp_resource_actions`
- `smartcmp_resource_actions_by_ids`

### Linux VM Discovery

This example shows the discovery chain for the built-in Linux VM catalog and the matching T-shirt sizing profiles.

```hcl
provider "smartcmp" {
  # Read endpoint and credentials from SMARTCMP_* environment variables.
}

data "smartcmp_published_catalogs" "linux_vm" {
  query = "Linux VM"
}

data "smartcmp_catalog_business_groups" "linux_vm" {
  catalog_id = data.smartcmp_published_catalogs.linux_vm.items[0].id
}

data "smartcmp_catalog_component" "compute" {
  source_key = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
}

data "smartcmp_resource_pools" "linux_vm" {
  business_group_id = data.smartcmp_catalog_business_groups.linux_vm.items[0].id
  source_key        = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
  node_type         = data.smartcmp_catalog_component.compute.type_name
}

data "smartcmp_profiles" "linux_vm" {
  provision_scope   = true
  catalog_id        = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
  node_template_name = "Compute"
  flavor_type       = "MACHINE"
}

output "linux_vm_catalog_id" {
  value = data.smartcmp_published_catalogs.linux_vm.items[0].id
}

output "linux_vm_profiles" {
  value = [for p in data.smartcmp_profiles.linux_vm.items : p.name]
}
```

### Action Discovery

Use these datasources to discover which day-two actions SmartCMP currently exposes before creating a `smartcmp_resource_operation`.

For deployment actions:

```hcl
data "smartcmp_deployment_actions" "example" {
  deployment_id = var.deployment_id
}

output "deployment_action_names" {
  value = [for item in data.smartcmp_deployment_actions.example.items : item.operation]
}
```

For VM resource actions:

```hcl
data "smartcmp_resource_actions" "vm" {
  resource_category = "iaas.machine"
  resource_id       = var.vm_resource_id
}

output "vm_actions" {
  value = [
    for item in data.smartcmp_resource_actions.vm.items : {
      operation   = item.operation
      name        = item.name
      schema_json = item.schema_json
    }
  ]
}
```

`smartcmp_resource_actions` also accepts `resource_ids` to return the common actions shared by multiple resources in the same category.

When you need per-resource action sets instead of the shared intersection, use `smartcmp_resource_actions_by_ids`:

```hcl
data "smartcmp_resource_actions_by_ids" "vm_batch" {
  resource_category = "iaas.machine"
  resource_ids      = var.vm_resource_ids
}

output "vm_actions_by_resource" {
  value = [
    for item in data.smartcmp_resource_actions_by_ids.vm_batch.items : {
      resource_id = item.resource_id
      actions     = [for action in item.actions : action.operation]
    }
  ]
}
```

Each discovered action returns stable typed metadata plus:

- `schema_json` for raw action parameter schema or input form metadata when SmartCMP exposes it
- `raw_json` for the untouched SmartCMP response object

## Resources

The provider exposes two request-oriented resources:

- `smartcmp_service_request`
- `smartcmp_resource_operation`

`smartcmp_service_request` submits a request through `/generic-request/submit`, tracks the resulting `GenericRequest`, and cancels the request during Terraform destroy when it is still non-terminal.

`smartcmp_resource_operation` triggers either deployment day-two operations or resource operations, tracks the resulting `GenericTask`, and cancels the task during Terraform destroy when it is still running.

The resources are intentionally request/task oriented. They do not model a VM or deployment lifecycle directly; they model the SmartCMP records returned by the platform APIs.

### Operating Existing VMs

Use `smartcmp_resource_operation` for day-two operations on resources that already exist in SmartCMP.

- Use `target_kind = "resource"` for a specific VM resource.
- Use `target_kind = "deployment"` for a deployment-level action.
- `operation` is passed through as-is. The provider does not translate `restart`, `powerOn`, `START_VM`, `resize`, or any other operation alias.
- `parameters_json` can be either:
  - a simple parameter map, which the provider wraps into `executeParameters` for resource actions or `params` for deployment actions
  - a full SmartCMP request body captured from the UI/API, which the provider forwards unchanged except for adding the selected `operation` and `target_id`

Typical VM restart example:

```hcl
resource "smartcmp_resource_operation" "vm_restart" {
  target_kind             = "resource"
  target_id               = var.vm_resource_id
  operation               = "restart"
  wait_for_terminal_state = true
}
```

Typical VM resize example:

```hcl
resource "smartcmp_resource_operation" "vm_resize" {
  target_kind             = "resource"
  target_id               = var.vm_resource_id
  operation               = "resize"
  wait_for_terminal_state = true

  parameters_json = jsonencode({
    computeProfileId = var.new_compute_profile_id
  })
}
```

The exact operation name and parameter shape depend on the SmartCMP action exposed by the target VM. Recommended discovery workflow:

1. Use `smartcmp_resource_actions` or `smartcmp_deployment_actions` to list the current operation identifiers.
2. Copy the selected `items[*].operation` value into `smartcmp_resource_operation.operation`.
3. Inspect `items[*].schema_json` and `items[*].raw_json` for the parameter contract.
4. If the platform still requires extra request details, inspect the UI request sent to `/deployments/{id}/day2-op` or `/nodes/resource-operations`.

Current limitations:

- Interactive `webOperation` actions are rejected because they do not return a trackable task.
- A `smartcmp_resource_operation` instance records one submitted task. To run the same restart or resize again, create a replacement task with `terraform apply -replace=...`.

### Service Request Mapping

`smartcmp_service_request` treats the request shell as stable and the service-specific resource model as dynamic.

Typed request-shell attributes cover the common contract:

- `catalog_id`
- `business_group_id`
- `name`
- `description`
- `project_id`
- `request_user_id`
- `request_count`
- `add_to_inventory`
- `resource_pool_id`
- `resource_pool_tags`
- `resource_pool_params_json`

Dynamic service-specific fields belong in either:

- `request_body_json` for request-level JSON such as `exts`, `tags`, `componentProperties`, and other SmartCMP fields
- `resource_specs[*].spec_json` for resource-model-specific fields such as `computeProfileId`, `logicTemplateId`, `templateId`, `params`, `networkSpecs`, or cloud-specific keys

When `resource_specs` is configured, the provider builds the final `resourceSpecs` array from the typed wrapper plus `spec_json`. It does not merge `request_body_json.resourceSpecs` by index. That conflict is rejected so the request shape stays predictable.

Service-specific fields must be discovered from SmartCMP itself. The recommended workflow is:

1. Open the target catalog in SmartCMP UI.
2. Open browser developer tools.
3. Submit or preview the request in the UI.
4. Inspect the payload sent to `catalogs/provision/deployment/...`.
5. Copy the service-specific fields into `request_body_json` or `resource_specs[*].spec_json`.

Example:

```hcl
resource "smartcmp_service_request" "linux_vm" {
  catalog_id        = "BUILD-IN-CATALOG-LINUX-VM"
  business_group_id = "replace-with-a-real-business-group-id"
  name              = "linux-vm-request"

  resource_pool_id = "replace-with-a-real-resource-bundle-id"

  resource_specs = [{
    node = "Compute"
    type = "cloudchef.nodes.Compute"
    spec_json = jsonencode({
      computeProfileId   = "replace-with-a-real-profile-id"
      logicTemplateId    = "replace-with-a-real-logic-template-id"
      credentialUser     = "root"
      credentialPassword = "replace-with-a-real-password"
      params = {
        custom_key = "custom-value"
      }
    })
  }]
}
```

### Service Request Test Matrix

The provider tests `smartcmp_service_request` at three levels:

- payload mapping unit tests
- resource create/delete behavior tests
- environment-gated acceptance smoke tests

The current mapping-focused unit scenarios cover:

- typed request-shell fields override conflicting `request_body_json` keys
- unknown `request_body_json` fields pass through unchanged
- typed `resource_specs[*]` wrappers override conflicting `spec_json` keys
- top-level `resource_pool_*` values fill missing resource-spec defaults
- explicit resource-spec values are not overwritten by top-level defaults
- invalid JSON object shapes are rejected for:
  - `request_body_json`
  - `resource_pool_params_json`
  - `resource_specs[*].spec_json`
- `resource_specs` conflicts with `request_body_json.resourceSpecs`
- multiple typed `resource_specs` require `node`

The resource behavior tests cover:

- submit payload generation for a mixed typed-plus-dynamic request
- create stores the returned request state
- read resolves linked deployment IDs
- delete cancels non-terminal requests
- create returns diagnostics for conflicting `resource_specs` inputs

Smoke acceptance coverage focuses on live reads that should work in a minimally configured environment:

- provider login and protected API read
- Linux VM catalog discovery
- Linux VM profile discovery
- catalog -> business group -> component -> cloud entry type -> resource pool/application discovery chain

Fixture-gated scenarios should remain optional because they depend on tenant-specific infrastructure:

- service request create against a real catalog with all required runtime parameters
- OS template and image lookup for environments that expose those concepts
- deployment-linked and resource-linked day-two operations

The current resource-operation unit scenarios cover:

- deployment actions that wrap a simple parameter map into `params`
- resource actions that wrap a simple parameter map into `executeParameters`
- full deployment action payload passthrough without extra wrapping
- full resource action payload passthrough without extra wrapping
- rejection of interactive `webOperation` actions

Recommended local commands:

```bash
go test ./internal/provider -run 'TestBuildServiceRequestPayload|TestServiceRequestResourceCreate' -count=1 -v
go test ./... -count=1
```

## Local Development

```bash
go test ./...
go build ./...
terraform fmt -recursive examples
```

To run the provider locally with debug support:

```bash
go run . -debug
```

## Acceptance Test Environment

Acceptance tests are environment-gated. The recommended variables are:

- `SMARTCMP_TEST_BASE_URL`
- `SMARTCMP_TEST_USERNAME`
- `SMARTCMP_TEST_PASSWORD`
- `SMARTCMP_TEST_TENANT_ID`
- `SMARTCMP_TEST_INSECURE`
- `SMARTCMP_TEST_CATALOG_NAME`
- `SMARTCMP_TEST_RESOURCE_BUNDLE_ID`
- `SMARTCMP_TEST_LOGIC_TEMPLATE_ID`
- `SMARTCMP_TEST_CLOUD_ENTRY_TYPE_ID`
- `SMARTCMP_TEST_DEPLOYMENT_ID`
- `SMARTCMP_TEST_RESOURCE_ID`
- `NO_PROXY`

Example:

```bash
export SMARTCMP_TEST_BASE_URL="https://smartcmp.example.com"
export SMARTCMP_TEST_USERNAME="replace-with-username"
export SMARTCMP_TEST_PASSWORD="replace-with-password"
export SMARTCMP_TEST_TENANT_ID="default"
export SMARTCMP_TEST_INSECURE="true"
export SMARTCMP_TEST_CATALOG_NAME="Linux VM"
export NO_PROXY="smartcmp.example.com"

go test ./internal/provider -run TestAcceptance -count=1 -v
```

Fixture-dependent tests should be skipped unless the bundle, template, image, deployment, and resource IDs are provided.

For environments where the endpoint is configured but unreachable from the current machine, the smoke acceptance tests are expected to skip cleanly instead of failing.
