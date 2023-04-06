terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.9.2"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.7"
    }
  }
  required_version = ">= 0.13"
}
