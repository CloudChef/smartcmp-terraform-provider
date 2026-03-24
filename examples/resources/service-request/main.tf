terraform {
  required_providers {
    smartcmp = {
      source  = "cloudchef/smartcmp"
      version = "~> 0.1"
    }
  }
}

variable "guest_password" {
  type        = string
  sensitive   = true
  description = "Guest OS credential password used by the catalog-specific payload."
}

provider "smartcmp" {
  # Read endpoint and credentials from SMARTCMP_* environment variables.
}

resource "smartcmp_service_request" "linux_vm" {
  catalog_id        = "BUILD-IN-CATALOG-LINUX-VM"
  business_group_id = "replace-with-a-real-business-group-id"
  name              = "linux-vm-request"

  resource_pool_id = "replace-with-a-real-resource-bundle-id"

  request_body_json = jsonencode({
    exts = {
      source = "terraform"
    }
  })

  resource_specs = [{
    node = "Compute"
    type = "cloudchef.nodes.Compute"
    spec_json = jsonencode({
      computeProfileId   = "replace-with-a-real-profile-id"
      logicTemplateId    = "replace-with-a-real-logic-template-id"
      credentialUser     = "root"
      credentialPassword = var.guest_password
      params = {
        custom_key = "custom-value"
      }
    })
  }]

  wait_for_terminal_state = false
}
