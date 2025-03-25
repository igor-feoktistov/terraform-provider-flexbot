---
page_title: "Flexbot Provider"
---

# Flexbot Provider

The [Flexbot](https://github.com/igor-feoktistov/flexbot) provider allows to build and manage bare-metal Linux on [FlexPod](https://flexpod.com) (Cisco UCS and NetApp cDOT).

Compared to other bare-metal tools it does not require any additional infrastructure like PXE/DHCP servers.

## Example - Base

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
      api_method = "rest"
    }
  }
}
```

## Example - `rancher_api` is enabled with provider type of `rancher2`

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
      api_method = "rest"
    }
  }
  # Rancher API (rancher2 provider)
  rancher_api {
    enabled = true
    provider = "rancher2"
    api_url = "https://rancher.example.com"
    token_key = "token-xxxx"
    insecure = true
    retries = 12
    cluster_name = rancher2_cluster.cluster.name
    cluster_id = rancher2_cluster.cluster.id
    node_grace_timeout = 60
    wait_for_node_timeout = 1800
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}
```

## Example - `rancher_api` is enabled with provider type of `rke`

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
      api_method = "rest"
    }
  }
  # Rancher API (rke provider)
  rancher_api {
    enabled = true
    provider = "rke"
    api_url = "https://rke.example.com:6443"
    cluster_name = "onprem-us-east-1-01"
    cluster_id = "onprem-us-east-1-01"
    server_ca_data = "LS0tLS1CEUdJUiBDRVJUSKZJQ0F<...skip...>tLSItYQo="
    client_cert_data = "LS0dLS1TRUdJTi<...skip...>BFEUaLS0sLQo="
    client_key_data = "base64:giZIN7H04oQw5<...skip...>8k4uoWEIg/woi2n3+4kQ="
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}
```

## Example - `rancher_api` is enabled with provider type of `rk-api`

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
      ext_attributes = {
        "Region" = "us-east-1"
        "Site" = "onprem-us-east-1-01"
      }
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
      api_method = "rest"
    }
  }
  # Rancher API (rk-api provider)
  rancher_api {
    enabled = true
    provider = "rk-api"
    api_url = "https://rancher.example.com"
    token_key = "token-xxxx"
    insecure = true
    retries = 12
    cluster_name = rancher2_cluster_v2.cluster.name
    cluster_id = rancher2_cluster_v2.cluster.cluster_v1_id
    node_grace_timeout = 60
    wait_for_node_timeout = 1800
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}
```

## Example - `rancher_api` is enabled with provider type of `harvester`

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
      ext_attributes = {
        "Region" = "us-east-1"
        "Site" = "onprem-us-east-1-01"
      }
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
      api_method = "rest"
    }
  }
  # Rancher API (harvester provider)
  rancher_api {
    enabled = true
    provider = "harvester"
    api_url = "https://harvester.example.com"
    token_key = "base64:wiYIN9H79fK2q<...skip...>tu5ucJDDg/swi4g1+6aQ="
    insecure = true
    retries = 12
    wait_for_node_timeout = 1800
  }
}
```

## Argument Reference

The following arguments are supported:

* `pass_phrase` - (Optional) Password phrase to decrypt passwords in credentials (if encrypted). See `flexbot_crypt` datasource example on how to generate encrypted user / password values.
* `pass_phrase_env_key` - (Optional) Environment variable to pass encryption key to decrypt `pass_phrase` (if encrypted). If `pass_phrase` is encrypted, machine ID is used as default password phrase unless `pass_phrase_env_key` is defined.
* `ipam` - (Required) IPAM is implemented via pluggable providers. Only "Infoblox" and "Internal" providers are supported at this time. "Internal" provider expects you to supply "ip" and "fqdn" in network configurations.
* `compute` - (Required) UCS compute, credentials to access UCSM
* `storage` - (Required) cDOT storage, credentials to access cDOT cluster or SVM
* `rancher_api` - (Optional) Rancher API helps with node management in Rancher, RKE, or Harvester cluster to ensure graceful node updates, shutdown, restarts, and removals.
* `synchronized_updates` - (Optional) Synchronized nodes updates. It is highly suggested to enable it when Rancher API is enabled. Enforces sequential and synchronized updates for Rancher cluster nodes.

#### `ipam`

##### Arguments

* `provider` - (Required) IPAM provider. Currently supported providers are `Infoblox` and `Internal`. Provider `Internal` means you are responsible for node IP's allocation (string).
* `credentials` - (Optional) Infoblox specific credentials parameters:
  * `host` - (Required) API endpoint host name or IP address (string).
  * `user` - (Required) Username, can be encrypted by `flexbot-crypt` (string).
  * `password` - (Required) Password, can be encrypted by `flexbot-crypt` (string).
  * `wapi_version` - (Required) WAPI version (string).
  * `dns_view` - (Required) Infoblox DNS View (string).
  * `network_view` - (Required) Infoblox Network View (string).
  * `ext_attributes` - (Optional) Infoblox Extensible Attributes (map[string][string]).
* `dns_zone` - (Optional) Default DNS zone for DNS records creation (string).

#### `compute`

##### Arguments

* `credentials` - (Optional) UCSM specific credentials parameters:
  * `host` - (Required) XML API endpoint host name or IP address
  * `user` - (Required) Username, can be encrypted by `flexbot-crypt` (string).
  * `password` - (Required) Password, can be encrypted by `flexbot-crypt` (string).

#### `storage`

##### Arguments

* `credentials` - (Required) ONTAP SVM or cluster specific credentials parameters:
  * `host` - (Required) SVM host name (IP address) for SVM scope or cDOT cluster name (IP address) for cluster scope (cluster scope is supported for `rest` only)
  * `user` - (Required) Username, can be encrypted by `flexbot-crypt` (string).
  * `password` - (Required) Password, can be encrypted by `flexbot-crypt` (string).
  * `api_method` - (Optional) ONTAP API method is either `zapi` or `rest`. Method `rest` requires ONTAP v9.12 or higher (string, default is `zapi`).
  * `zapi_version` - (Optional) Typically not required except some old ONTAP releases. Will be deprecated in the future (string).

#### `rancher_api`

##### Arguments

* `enabled` - (Optional) Quickly enable/disable rancher API support (bool, default is `false`)
* `provider` - (Optional) Rancher API provider. Currently supported `rancher2`, `rke`, and `rk-api` when in mix with respective terraform providers (string, defailt is `rancher2`).
  * `rancher2` - manages RKE1 and RKE2 downstream cluster nodes (implemeneted via legacy Norman API)
  * `rk-api` - manages RKE2 downstream cluster nodes (implemented via RK API and requires Rancher Management Server v2.8.5 or higher)
  * `rke` - manages RKE1 clusters outside of Rancher Management Server (implemented via standard Kubernetes API to cordon/drain nodes and maintain annotations, labels, and taints)
  * `rke2` - manages RKE2 clusters outside of Rancher Management Server (implemented via standard Kubernetes API to cordon/drain/delete nodes and maintain annotations, labels, and taints)
* `api_url` - (Required) Rancher API endpoint is either Rancher Server endpoint or Kubernetes API endpoint for RKE/Kubernetes use case (string).
* `cluster_name` - (Required) Kubernetes cluster name (string).
* `cluster_id` - (Required) Downstream cluster ID in case of `rancher2`, or Kubernetes cluster name (string).
* `token_key` - (Optional) API token for Rancher API, required for `rancher2` provider. Can be encrypted by `flexbot-crypt` (string).
* `server_ca_data` - (Optional) Server CA, base64 encoded PEM, exactly as you would have it in kubeconfig. Can be encrypted by `flexbot-crypt` (string)
* `client_cert_data` - (Optional) Client certificate for x509 authentication, base64 encoded PEM, exactly as you would have it in kubeconfig. Can be encrypted by `flexbot-crypt` (string)
* `client_key_data` - (Optional) Client private key for x509 authentication, base64 encoded PEM, exactly as you would have it in kubeconfig. Can be encrypted by `flexbot-crypt` (string)
* `insecure` - (Optional) Disable certificate verification (bool, default is `false`).
* `retries` - (Optional) Number of API calls retries in case of transient errors (int, default is 3 with 5 seconds wait between attempts)
* `node_grace_timeout` - (Optional) Wait after node update is completed (int, seconds, default is 0).
* `wait_for_node_timeout` - (Optional) MAX wait time until node is available (int, seconds, default is 0).
* `drain_input` - (Optional) Drain operation parameters (map).
