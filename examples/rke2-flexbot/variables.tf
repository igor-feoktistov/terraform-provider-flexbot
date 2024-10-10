variable "nodes" {
  type = map(object({
    blade_spec = object({
      dn = string
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
      ssh_public_key_ecdsa = string
      ssh_private_key_ecdsa = string
    })
    network = map(list(object({
      name = string
      subnet = string
      gateway = string
      dns_server1 = string
      dns_server2 = string
      dns_server3 = string
      dns_domain = string
      parameters = map(string)
    })))
    storage = object({
      api_method = string
    })
  })
}

variable "rke2_config" {
  type = object({
    rke2_cluster = string
    rke2_token = string
    rke2_version = string
    rke2_server = string
    server_ca_data = string
    client_cert_data = string
    client_key_data = string
    s3_backup = object({
      region = string
      endpoint = string
      access_key_id = string
      secret_access_key = string
      bucket = string
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
