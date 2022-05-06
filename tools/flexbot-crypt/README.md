# flexbot-crypt

`flexpod-crypt` is a simple tool to encrypt strings for use in `terraform-provider-flexbot`.

## Usage

```flexbot --passphrase=<password phrase> --sourceString <string to encrypt>```
 
 or

```flexbot --passphrase=<password phrase>```

to read string from STDIN

 or just

```flexbot```

to read string from STDIN and use machineID for passphrase
