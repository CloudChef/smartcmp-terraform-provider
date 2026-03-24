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

variable "new_compute_profile_id" {
  type        = string
  description = "Target SmartCMP compute profile ID for the resize action."
}

resource "smartcmp_resource_operation" "vm_resize" {
  target_kind             = "resource"
  target_id               = var.vm_resource_id
  operation               = "resize"
  wait_for_terminal_state = true

  parameters_json = jsonencode({
    computeProfileId = var.new_compute_profile_id
  })
}
