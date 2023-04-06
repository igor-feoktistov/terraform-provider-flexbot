terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = "=1.9.2"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = "=1.25.0"
    }
  }
  required_version = ">=0.15"
}
