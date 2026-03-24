terraform {
  required_providers {
    smartcmp = {
      source  = "cloudchef/smartcmp"
      version = "~> 0.1"
    }
  }
}

variable "prod_base_url" {
  type        = string
  description = "SmartCMP base URL for the production tenant."
}

variable "prod_username" {
  type        = string
  description = "SmartCMP username for the production tenant."
}

variable "prod_password" {
  type        = string
  sensitive   = true
  description = "SmartCMP password for the production tenant."
}

variable "prod_tenant_id" {
  type        = string
  description = "SmartCMP tenant ID for the production tenant."
}

variable "dev_base_url" {
  type        = string
  description = "SmartCMP base URL for the development tenant."
}

variable "dev_username" {
  type        = string
  description = "SmartCMP username for the development tenant."
}

variable "dev_password" {
  type        = string
  sensitive   = true
  description = "SmartCMP password for the development tenant."
}

variable "dev_tenant_id" {
  type        = string
  description = "SmartCMP tenant ID for the development tenant."
}

provider "smartcmp" {
  base_url  = var.prod_base_url
  username  = var.prod_username
  password  = var.prod_password
  tenant_id = var.prod_tenant_id
  # Set auth_mode = "private" when a private deployment uses a SmartCMP-owned domain.
}

provider "smartcmp" {
  alias     = "dev"
  base_url  = var.dev_base_url
  username  = var.dev_username
  password  = var.dev_password
  tenant_id = var.dev_tenant_id
}
