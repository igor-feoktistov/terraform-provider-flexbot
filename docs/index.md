---
page_title: "Flexbot Provider"
---

# Flexbot Provider

The [Flexbot](https://github.com/igor-feoktistov/flexbot) provider allows to build bare-metal Linux on [FlexPod](https://flexpod.com) (Cisco UCS and NetApp cDOT).

Compared to other bare-metal tools it does not require any additional infrastructure like PXE/DHCP servers.

## Example Usage

```hcl
provider "flexbot" {
  pass_phrase = "secret"
  synchronized_updates = true
  # IPAM
  ipam {
    provider = "Infoblox"
    credentials {
      host = "ib.example.com"
      user = "admin"
      password = "base64:jqdbcMI8dI5Dq<...skip...>yoskcRz9UUP+gN4v0Eo="
      wapi_version = "2.5"
      dns_view = "Internal"
      network_view = "default"
    }
    dns_zone = "example.com"
  }
  # UCS compute
  compute {
    credentials {
      host = "ucsm.example.com"
      user = "admin"
      password = "base64:kEqDbvk/DwABc<...skip...>orS6UIjo21DpA6QTFDOc="
    }
  }
  # cDOT storage
  storage {
    credentials {
      host = "vserver.example.com"
      user = "vsadmin"
      password = "base64:qiZIN5H04oK15<...skip...>7k4uoBIIg/boi2n3+4kQ="
    }
  }
  # Rancher API
  rancher_api {
    api_url = "https://rancher.example.com"
    token_key = "token-xxxx"
    insecure = true
    cluster_id = rancher2_cluster.cluster.id
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 60
      ignore_daemon_sets = true
      timeout = 1800
    }
  }
}
```

## Argument Reference

The following arguments are supported:

* `pass_phrase` - (Optional) Password phrase to decrypt passwords in credentials (if encrypted). Use 'flexbot --op=encryptString [--passphrase=<password phrase>]' CLI to generate encrypted passwords values.
* `ipam` - (Required) IPAM is implemented via pluggable providers. Only "Infoblox" and "Internal" providers are supported at this time. "Internal" provider expects you to supply "ip" and "fqdn" in network configurations.
* `compute` - (Required) UCS compute, credentials to access UCSM
* `storage` - (Required) cDOT storage, credentials to access cDOT cluster or SVM
* `rancher_api` - (Optional) Rancher API helps with node management in Rancher cluster to ensure graceful node updates and removals.
* `synchronized_updates` - (Optional) Synchronized nodes updates. It is highly suggested to enable it when Rancher API is enabled. Enforces sequential and synchronized updates for Rancher cluster nodes.
