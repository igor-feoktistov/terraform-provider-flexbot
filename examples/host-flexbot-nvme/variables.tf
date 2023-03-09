variable "nodes" {
  type = map(object({
    blade_spec = object({
      dn = string
      model = string
      total_memory = string
    })
    powerstate = string
    os_image = string
    seed_template = string
    boot_lun_size = number
    data_nvme_size = number
    restore = object({
      restore = bool
      snapshot_name = string
    })
    snapshots = list(object({
      name = string
      fsfreeze = bool
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
      ssh_public_key_path = string
      ssh_private_key_path = string
    })
    network = map(list(object({
      name = string
      subnet = string
      gateway = string
      dns_server1 = string
      dns_server2 = string
      dns_domain = string
    })))
    nvme_hosts = list(object({
      host_interface = string
    }))
    storage = object({
      api_method = string
    })
  })
}

variable "pass_phrase" {
  type = string
}
