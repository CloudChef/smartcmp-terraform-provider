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

variable "vm_resource_id" {
  type        = string
  description = "Existing SmartCMP VM resource ID."
}

resource "smartcmp_resource_operation" "vm_restart" {
  target_kind             = "resource"
  target_id               = var.vm_resource_id
  operation               = "restart"
  wait_for_terminal_state = true
}
