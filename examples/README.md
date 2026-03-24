# Examples

This directory contains small, copyable examples for the SmartCMP Terraform provider.

The examples intentionally avoid hard-coding passwords or internal endpoints. Set `SMARTCMP_*` environment variables, or pass sensitive values through Terraform variables, before running them.

- `providers/` shows provider configuration and aliases
- `datasources/linux-vm/` shows the published catalog and profile sizing workflow
- `datasources/deployment-actions/` shows how to discover deployment day-two actions
- `datasources/resource-actions/` shows how to discover resource day-two actions
- `datasources/resource-actions-by-ids/` shows how to discover actions for each resource in a batch
- `resources/service-request/` shows the stable request shell plus dynamic resource-spec mapping
- `resources/resource-operation/` shows the day-two operation contract
- `resources/vm-restart/` shows a restart task for an existing VM resource
- `resources/vm-resize/` shows a resize task for an existing VM resource
