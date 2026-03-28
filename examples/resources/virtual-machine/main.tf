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

variable "credential_password" {
  description = "Initial login password for the provisioned VM."
  type        = string
  sensitive   = true
  default     = "Sample_Password"
}

locals {
  catalog_id         = "replace-with-a-real-catalog-id"
  business_group_id  = "replace-with-a-real-business-group-id"
  request_user_id    = "replace-with-a-real-request-user-id"
  resource_pool_id   = "replace-with-a-real-resource-bundle-id"
  compute_profile_id = "replace-with-a-real-compute-profile-id"
  logic_template_id  = "replace-with-a-real-logic-template-id"
}

resource "smartcmp_virtual_machine" "example" {
  catalog_id        = local.catalog_id
  business_group_id = local.business_group_id
  request_user_id   = local.request_user_id
  resource_pool_id  = local.resource_pool_id
  name              = "tf-vm-example"
  description       = "Terraform-managed SmartCMP VM"

  instance_type       = "ecs.t6-c1m1.large"
  cpu                 = 2
  memory_gb           = 2
  power_state         = "started"
  compute_profile_id  = local.compute_profile_id
  logic_template_id   = local.logic_template_id
  credential_user     = "root"
  credential_password = var.credential_password

  # Change power_state to "stopped" and run terraform apply to stop the VM.
  # Change instance_type/cpu/memory_gb and run terraform apply again to trigger stop -> resize.
  start_after_resize = false

  timeouts = {
    create = "30m"
    update = "30m"
    delete = "30m"
  }
}

output "virtual_machine_resource_id" {
  value = smartcmp_virtual_machine.example.id
}

output "virtual_machine_deployment_id" {
  value = smartcmp_virtual_machine.example.deployment_id
}

output "virtual_machine_status" {
  value = smartcmp_virtual_machine.example.status
}

output "virtual_machine_power_state" {
  value = smartcmp_virtual_machine.example.power_state
}

output "virtual_machine_instance_type_actual" {
  value = smartcmp_virtual_machine.example.instance_type_actual
}
