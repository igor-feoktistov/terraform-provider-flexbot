Terraform Flexbot Provider
==========================

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

Flexbot provider is a tool to build and manage bare-metal Linux on [FlexPod](https://flexpod.com) (Cisco UCS and NetApp cDOT).

Compared to other bare-metal tools it does not require any additional infrastructure like PXE/DHCP servers.

The provider is integrated with Rancher API for Rancher downstream clusters nodes lifecycle management.


Requirements
------------

- [Terraform](https://www.terraform.io/downloads.html) 0.15.x or later
- [Go](https://golang.org/doc/install) 1.19 or later


Building the provider
---------------------

* Clone [terraform-provider-flexbot project repository](https://github.com/igor-feoktistov/terraform-provider-flexbot) to: `$GOPATH/src`.
* Enter `$GOPATH/src/terraform-provider-flexbot` directory and run `make` to build the provider.


Using the provider
------------------
If you want to use the pre-built binaries published on [registry.terraform.io](https://registry.terraform.io/providers/igor-feoktistov/flexbot), just run `terraform init`.
For the provider built from a source code, follow the instructions to [install it as a plugin](https://www.terraform.io/docs/language/providers/requirements.html).
After placing it into your plugins directory, run `terraform init` to initialize it. Please see the examples in [examples](https://github.com/igor-feoktistov/terraform-provider-flexbot/tree/master/examples) directory.
