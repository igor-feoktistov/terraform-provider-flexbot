---
page_title: "Flexbot Configuration Examples"
---

# Flexbot Configuration Examples

Here you can find the examples that may help you to jumpstart with `flexbot` provider for different use cases.
Make sure to update respective `terraform.tfvars` files with your own infrastructure configuration settings.

## Examples

* [simple](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples/simple) Simple configuration with a lot of comments.
* [host-flexbot](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples/host-flexbot) Provisions multiple servers the same configuration in one shot.
* [rke-flexbot](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples/rke-flexbot) Provisions RKE cluster.
* [rancher-server-flexbot](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples/rancher-server-flexbot) Provisions Rancher Server on top of RKE cluster.
* [rancher-workload-cluster-flexbot](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples/rancher-workload-cluster-flexbot) Provisions Rancher workload cluster.

### Note
You can easily adapt the examples with IPAM provider via Terraform.
In `flexbot` provider confguration use the following `ipam` definition to disable built-in provider:
```
  ipam {
    provider = "Internal"
  }
```
Then you need to supply `ip` and `fqdn` in resource network for `node` and `iscsi-initiator`.
