terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.9.4"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = ">=1.24.1"
    }
  }
  required_version = ">=0.15"
}
