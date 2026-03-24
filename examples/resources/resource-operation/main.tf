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

resource "smartcmp_resource_operation" "power_on" {
  target_kind = "deployment"
  target_id   = "replace-with-a-deployment-id"
  operation   = "powerOn"
  comment     = "Start the deployment after approval."
  parameters_json = jsonencode({
    reason = "requested-by-terraform"
  })
}
