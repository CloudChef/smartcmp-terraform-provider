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

variable "resource_ids" {
  type        = list(string)
  description = "Existing SmartCMP resource IDs in the same category."
}

data "smartcmp_resource_actions_by_ids" "vm_batch" {
  resource_category = "iaas.machine"
  resource_ids      = var.resource_ids
}

output "vm_actions_by_resource" {
  value = [
    for item in data.smartcmp_resource_actions_by_ids.vm_batch.items : {
      resource_id = item.resource_id
      actions = [
        for action in item.actions : {
          operation = action.operation
          name      = action.name
        }
      ]
    }
  ]
}
