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

variable "resource_id" {
  type        = string
  description = "Existing SmartCMP resource ID."
}

data "smartcmp_resource_actions" "vm" {
  resource_category = "iaas.machine"
  resource_id       = var.resource_id
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
