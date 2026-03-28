terraform {
  required_providers {
    smartcmp = {
      source  = "cloudchef/smartcmp"
      version = "~> 0.1"
    }
  }
}

provider "smartcmp" {
  auth_mode = "private"
}

variable "credential_password" {
  description = "Initial root password for the provisioned VM. Pass it with TF_VAR_credential_password or -var."
  type        = string
  sensitive   = true
  default     = "Sample_Password"
}

locals {
  request_name         = "tf-linux-success-20260328013340"
  catalog_id           = "1ff5ae45-bd30-42ed-b2b8-0a17fb7eed57" # Cloned Linux VM catalog
  business_group_id    = "922f3a25-a5c4-48f4-9c94-62aae02e4266" # School A
  request_user_id      = "9834cf8b-af1e-4c5b-99f3-0ed3080bc5ed" # tenant_user
  resource_bundle_id   = "5eadd756-d269-4570-b7fc-46139f7a2dad" # Huawei FCResource Bundle
  compute_profile_id   = "2a050924-817c-40c6-b3ff-ec5253fb3e13" # Tiny Computing
  logic_template_id    = "e6ec1731-f81f-4878-ba67-f31f2bae0841" # CentOS
  physical_template_id = "8e9e3f25-af76-4a7b-aa46-0e2298112b0a"
  template_id          = "urn:sites:E4910EDA:vms:i-00005E09"
  datastore_id         = "urn:sites:E4910EDA:datastores:2"
  network_id           = "urn:sites:E4910EDA:dvswitchs:3"
  subnet_id            = "urn:sites:E4910EDA:dvswitchs:3:portgroups:4"
  security_group_id    = "16"
}

resource "smartcmp_service_request" "linux_vm_success" {
  catalog_id        = local.catalog_id
  business_group_id = local.business_group_id
  request_user_id   = local.request_user_id
  resource_pool_id  = local.resource_bundle_id
  name              = local.request_name
  description       = "Terraform success probe for Linux VM clone catalog"

  resource_specs = [
    {
      node = "Compute"
      type = "cloudchef.nodes.Compute"
      spec_json = jsonencode({
        computeProfileId   = local.compute_profile_id
        flavorId           = local.compute_profile_id
        logicTemplateId    = local.logic_template_id
        physicalTemplateId = local.physical_template_id
        templateId         = local.template_id

        credentialUser     = "root"
        credentialPassword = var.credential_password

        systemDisk = {
          size           = 40
          is_system_disk = true
          volume_type    = local.datastore_id
          disk_policy    = "type"
          disk_tags      = []
        }

        dataDisks = [
          {
            name        = "data01"
            size        = 20
            volume_type = local.datastore_id
            disk_policy = "type"
            disk_tags   = []
          }
        ]

        networkId = local.network_id
        subnetId  = local.subnet_id

        securityGroupIds = [local.security_group_id]

        params = {
          use_password    = false
          vm_display_name = ""
          hostname        = ""
          folder          = ""
        }
      })
    }
  ]

  wait_for_terminal_state = true

  timeouts = {
    create = "25m"
  }
}

output "request_id" {
  value = smartcmp_service_request.linux_vm_success.id
}

output "request_name" {
  value = smartcmp_service_request.linux_vm_success.name
}

output "request_state" {
  value = smartcmp_service_request.linux_vm_success.state
}

output "error_message" {
  value = smartcmp_service_request.linux_vm_success.error_message
}

output "deployment_ids" {
  value = smartcmp_service_request.linux_vm_success.deployment_ids
}
