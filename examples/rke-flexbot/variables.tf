variable "nodes" {
  type = map(object({
    blade_spec = object({
      dn = string
      model = string
      total_memory = string
    })
    ip = string
    powerstate = string
    os_image = string
    seed_template = string
    boot_lun_size = number
    data_lun_size = number
    restore = object({
      restore = bool
      snapshot_name = string
    })
    maintenance = object({
      execute = bool
      synchronized_run = bool
      tasks = list(string)
    })
    snapshots = list(object({
      name = string
      fsfreeze = bool
    }))
    labels = map(any)
    taints = list(object({
      key = string
      value = string
      effect = string
    }))
    rke_member = bool
  }))
}

variable "flexbot_credentials" {
  type = map(object({
    host = string
    user = string
    password = string
  }))
}

variable "node_config" {
  type = object({
    infoblox = object({
      wapi_version = string
      dns_view = string
      network_view = string
      dns_zone = string
    })
    compute = object({
      sp_org = string
      sp_template = string
      ssh_user = string
      ssh_public_key = string
      ssh_private_key = string
    })
    network = map(list(object({
      name = string
      subnet = string
      gateway = string
      dns_server1 = string
      dns_server2 = string
      dns_server3 = string
      dns_domain = string
    })))
    storage = object({
      api_method = string
      zapi_version = string
    })
  })
}

variable "rke_config" {
  type = object({
    cluster_name = string
    rke_version = string
    docker_version = string
    rancher_server = string
    api_url = string
    server_ca_data = string
    client_cert_data = string
    client_key_data = string
    s3_backup = object({
      region = string
      endpoint = string
      access_key_id = string
      secret_access_key = string
      bucket_name = string
      folder = string
    })
  })
}

variable "pass_phrase" {
  description = "Password to unlock credentials and keys"
  type = string
  sensitive = true
}

variable "output_path" {
  type = string
  description = "Path to output directory"
}
