# SmartCMP Operation Discovery Datasource Design

## Summary

The provider can already execute SmartCMP day-two operations through `smartcmp_resource_operation`, but it does not yet help users discover which actions are available for a specific deployment or VM resource.

This design adds discovery-only data sources so users can enumerate supported operations and inspect the raw schema before building `smartcmp_resource_operation` payloads.

## Goals

- List the allowed actions for an existing deployment or resource.
- Surface whether an action is interactive (`webOperation`) or non-interactive.
- Return raw payload metadata so Terraform users can copy the platform contract without guessing.
- Keep execution separate from discovery. `smartcmp_resource_operation` remains the only mutation resource.

## Proposed Interfaces

### `smartcmp_deployment_actions`

Inputs:

- `deployment_id`

Outputs per item:

- `operation`
- `name`
- `display_name`
- `description`
- `web_operation`
- `schema_json`
- `raw_json`

### `smartcmp_resource_actions`

Inputs:

- `resource_id`
- optional `deployment_id`
- optional `node_id`

Outputs per item:

- `operation`
- `name`
- `display_name`
- `description`
- `web_operation`
- `schema_json`
- `raw_json`

## Recommended Implementation Strategy

1. Discover the backend endpoints used by the UI for existing deployment and resource action menus.
2. Validate those endpoints against SmartCMP service interfaces and Swagger where available.
3. Prefer returning stable typed metadata plus `raw_json` instead of normalizing action-specific fields too aggressively.
4. Treat action-parameter schemas as opaque JSON unless a stable cross-platform contract exists.

## Notes

- Operation names vary by component and platform. Examples seen in local docs and code include `restart`, `reboot`, `START_VM`, `powerOn`, and `resize`.
- Some actions may be exposed in UI as interactive-only workflows. These should still appear in discovery results with `web_operation = true`, even though execution remains unsupported by the Terraform mutation resource.
- This should be implemented as a follow-up feature. The current provider already supports direct execution when users know the operation name and parameter shape.
