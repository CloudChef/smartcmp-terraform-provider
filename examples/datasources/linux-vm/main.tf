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

data "smartcmp_published_catalogs" "linux_vm" {
  query = "Linux VM"
}

data "smartcmp_catalog_business_groups" "linux_vm" {
  catalog_id = data.smartcmp_published_catalogs.linux_vm.items[0].id
}

data "smartcmp_catalog_component" "compute" {
  source_key = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
}

data "smartcmp_resource_pools" "linux_vm" {
  business_group_id = data.smartcmp_catalog_business_groups.linux_vm.items[0].id
  source_key        = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
  node_type         = data.smartcmp_catalog_component.compute.type_name
}

data "smartcmp_profiles" "linux_vm" {
  provision_scope    = true
  catalog_id         = data.smartcmp_published_catalogs.linux_vm.items[0].source_key
  node_template_name = "Compute"
  flavor_type        = "MACHINE"
}

output "catalog_name" {
  value = data.smartcmp_published_catalogs.linux_vm.items[0].name
}

output "profile_names" {
  value = [for profile in data.smartcmp_profiles.linux_vm.items : profile.name]
}
