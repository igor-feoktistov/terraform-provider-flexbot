# Flexbot to build and manage RKE cluster

This example demonstrates how to use `flexbot` and `rancher/rke` Terraform providers
to build and manage RKE cluster with bare-metal nodes on FlexPOD.

See [crypt](../crypt) example on how to generate encrypted strings for passwords and tokens.

To unlock encrypted passwords you can input `pass_phrase` value either via Terraform prompt
or other means like `.auto.tfvars` or ENV variable.

You will need to update `rke_config` in `terraform.auto.tfvars` after cluster creation.
Just update `server_ca_data`, `client_cert_data`, and `client_key_data` captured from generated in
output directory kubeconfig. Then just enable API in `rancher_api` of `main.tf` and you will be able
to manage node labels and taints as well as to perform graceful nodes shutdowns/restarts/migrations.
