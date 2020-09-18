terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">= 1.3.3"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.1"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = ">= 1.10.3"
    }
  }
  required_version = ">= 0.13"
}