# Service Request Mapping Design

## Summary

The provider must treat `smartcmp_service_request` as a request-shell resource with dynamic service-specific payloads. Only a small set of stable request fields and a small set of stable resource-default fields should be first-class Terraform attributes. Everything else must pass through JSON without the provider imposing a fixed VM-oriented or resource-model-specific schema.

This design reflects both the SmartCMP service-request documentation and the observed live Linux VM behavior:

- request-shell fields are stable across services
- resource-model fields vary by catalog and component
- Linux VM does not universally require template or image fields
- service-specific parameters should be discovered from SmartCMP UI or API payloads and passed through unchanged

## Goals

- Keep Terraform ergonomic for the stable request shell
- Avoid hard-coding VM-only assumptions into the provider
- Support single-node and multi-node service requests
- Allow provider-managed defaults for common resource-pool fields
- Preserve unknown service-specific fields exactly as provided by users

## Non-Goals

- Do not model every possible SmartCMP resource parameter as Terraform typed attributes
- Do not validate service-private fields such as `computeProfileId`, `logicTemplateId`, `templateId`, `params`, or `networkSpecs`
- Do not merge typed `resource_specs` with `request_body_json.resourceSpecs` by index

## Public Shape

### Stable request-shell attributes

These stay first-class in `smartcmp_service_request`:

- `catalog_id`
- `business_group_id`
- `name`
- `description`
- `project_id`
- `request_user_id`
- `count`
- `add_to_inventory`
- `wait_for_terminal_state`
- `timeouts`

### Stable resource-default attributes

These are optional top-level defaults used only for resource-spec fallback:

- `resource_pool_id`
- `resource_pool_tags`
- `resource_pool_params_json`

### Dynamic payload attributes

- `request_body_json`
  - JSON object for request-level dynamic fields such as `params`, `requestParameters`, `exts`, `tags`, `componentProperties`, `attachments`, and provider-unknown fields
- `resource_specs`
  - optional typed wrapper list for resource specs
  - each item contains:
    - `node`
    - `type`
    - `resource_pool_id`
    - `resource_pool_tags`
    - `resource_pool_params_json`
    - `spec_json`

## Mapping Rules

### Request-level merge order

1. Parse `request_body_json` as the base object.
2. Overlay stable typed request attributes:
   - `catalog_id -> catalogId`
   - `business_group_id -> businessGroupId`
   - `name -> name`
   - `description -> description`
   - `project_id -> projectId`
   - `request_user_id -> userId`
   - `count -> count`
   - `add_to_inventory -> addToInventory`
3. Keep unknown fields from `request_body_json` untouched.

### Resource-spec generation

If typed `resource_specs` is configured:

1. Reject `request_body_json.resourceSpecs` to avoid ambiguous array merge rules.
2. Build the final `resourceSpecs` array from typed `resource_specs`.
3. For each entry:
   - parse `spec_json` as the base object
   - overlay typed wrapper fields:
     - `node -> node`
     - `type -> type`
     - `resource_pool_id -> resourceBundleId`
     - `resource_pool_tags -> resourceBundleTags`
     - `resource_pool_params_json -> resourceBundleParams`
   - fill missing resource-pool fields from top-level defaults:
     - top-level `resource_pool_id`
     - top-level `resource_pool_tags`
     - top-level `resource_pool_params_json`

If typed `resource_specs` is not configured:

1. Use `request_body_json.resourceSpecs` as-is when present.
2. Optionally fill missing `resourceBundleId`, `resourceBundleTags`, and `resourceBundleParams` from top-level defaults.
3. Never overwrite a field that is already explicitly set in JSON.

## Validation Rules

The provider validates only the stable shell and JSON shapes.

### Required validations

- `catalog_id` must be set
- `business_group_id` must be set
- `name` must be set
- `request_body_json`, when set, must decode to a JSON object
- `resource_pool_params_json`, when set, must decode to a JSON object
- `resource_specs[*].spec_json`, when set, must decode to a JSON object

### Resource-spec validations

- if more than one typed `resource_specs` item exists, every item must set `node`
- `type` remains optional at provider level because service behavior varies
- provider must not enforce VM-specific fields such as:
  - `computeProfileId`
  - `logicTemplateId`
  - `templateId`
  - `networkId`
  - `credentialUser`
  - `params`

Those validations belong to SmartCMP itself.

## Documentation Guidance

The README and examples must state clearly:

- only request-shell fields are fixed in Terraform schema
- service-specific resource fields must be captured from SmartCMP UI or API traffic
- users should inspect `catalogs/provision/deployment/...` submissions, especially:
  - `requestParameters`
  - `extensibleParametersList`
  - the final submitted request payload

## Example Direction

The provider should document four patterns:

- minimal shell plus `request_body_json`
- Linux VM with typed `resource_specs` wrapper and `spec_json`
- non-VM service request with service-specific `params`
- discovery chain showing how to find catalog, business group, resource pool, and profiles before submit

## Testing Direction

Coverage should prove:

- typed request fields override `request_body_json`
- typed resource-spec wrappers override `spec_json`
- top-level resource defaults only fill missing fields
- typed `resource_specs` conflicts with `request_body_json.resourceSpecs`
- unknown fields pass through unchanged
- acceptance tests do not hard-code template or image requirements for Linux VM
