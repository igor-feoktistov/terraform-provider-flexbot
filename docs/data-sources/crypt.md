--
page_title: "flexbot_crypt Data Source"
---

# flexbot_crypt Data Source

Use this data source to retrieve decrypted token and access keys.
`flexbot_crypt` uses AES encryption with 256-bit keys generated via SHA256 sum.

## Example Usage

```
provider "flexbot" {
  alias = "crypt"
  pass_phrase = var.pass_phrase
}

data "flexbot_crypt" "rancher_token_key" {
  provider = flexbot.crypt
  name = "rancher_token_key"
  encrypted = var.rancher_config.token_key
}

provider "rancher2" {
  api_url = var.rancher_config.api_url
  token_key = data.flexbot_crypt.rancher_token_key.decrypted
  insecure = true
}
```

## Argument Reference

* `name` - (Required) The name encrypted entity (string)
* `encrypted` - (Optional/Computed) Encrypted string value (string)
* `decrypted` - (Optional/Computed) Decrypted string value (string)
