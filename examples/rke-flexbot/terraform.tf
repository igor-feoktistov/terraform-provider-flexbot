terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.8.0"
    }
    rke = {
      source = "rancher/rke"
      version = "= 1.3.2"
    }
  }
  required_version = ">= 0.15"
}
