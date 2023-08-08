terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = "=1.9.3"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = "=3.0.0"
    }
  }
  required_version = ">=1.5.4"
}
