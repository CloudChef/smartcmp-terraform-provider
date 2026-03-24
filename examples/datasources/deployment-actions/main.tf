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

variable "deployment_id" {
  type        = string
  description = "Existing SmartCMP deployment ID."
}

data "smartcmp_deployment_actions" "example" {
  deployment_id = var.deployment_id
}

output "deployment_actions" {
  value = [
    for item in data.smartcmp_deployment_actions.example.items : {
      operation     = item.operation
      name          = item.name
      web_operation = item.web_operation
    }
  ]
}
