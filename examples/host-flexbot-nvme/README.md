# Flexbot linux host with data disk on NVME example

This example demonstrates simple use case of single or multiple linux bare-metal hosts on FlexPOD with data disk on NVME.

See [crypt](../crypt) example on how to generate encrypted strings for passwords and tokens.

To unlock encrypted passwords you can input `pass_phrase` value either via Terraform prompt
or other means like `.auto.tfvars` or ENV variable.
