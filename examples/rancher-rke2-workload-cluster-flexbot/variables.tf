variable "nodes" {
  type = object({
    masters = map(object({
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
      data_lun_size = optional(number, 0)
      data_nvme_size = optional(number, 0)
      restore = optional(object({
        restore = optional(bool, false)
        snapshot_name = string
      }),
      {
        restore = false
        snapshot_name = ""
      })
      maintenance = optional(object({
        execute = optional(bool, false)
        synchronized_run = optional(bool, false)
        wait_for_node_timeout = optional(number, 0)
        node_grace_timeout = optional(number, 0)
        tasks = optional(list(string), ["cordon", "drain", "restart","uncordon"])
      }),
      {
        execute = false
        synchronized_run = false
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon", "drain", "restart","uncordon"]
      })
      snapshots = optional(list(object({
        name = string
        fsfreeze = optional(bool, false)
      })), [])
      labels = optional(map(any), {})
      taints = optional(list(object({
        key = string
        value = string
        effect = string
      })), [])
      force_update = optional(bool, false)
    }))
    workers = map(object({
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
      data_lun_size = optional(number, 0)
      data_nvme_size = optional(number, 0)
      restore = optional(object({
        restore = optional(bool, false)
        snapshot_name = string
      }),
      {
        restore = false
        snapshot_name = ""
      })
      maintenance = optional(object({
        execute = optional(bool, false)
        synchronized_run = optional(bool, false)
        wait_for_node_timeout = optional(number, 0)
        node_grace_timeout = optional(number, 0)
        tasks = optional(list(string), ["cordon", "drain", "restart","uncordon"])
      }),
      {
        execute = false,
        synchronized_run = false
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon", "drain", "restart","uncordon"]
      })
      snapshots = optional(list(object({
        name = string
        fsfreeze = optional(bool, false)
      })), [])
      labels = optional(map(any), {})
      taints = optional(list(object({
        key = string
        value = string
        effect = string
      })), [])
      force_update = optional(bool, false)
    }))
  })
}

variable "flexbot_credentials" {
  type = object({
    infoblox = object({
      host = string
      user = string
      password = string
    })
    ucsm = object({
      host = string
      user = string
      password = string
    })
    cdot = object({
      host = string
      user = string
      password = string
    })
  })
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
      gateway = optional(string, "")
      dns_server1 = optional(string, "")
      dns_server2 = optional(string, "")
      dns_server3 = optional(string, "")
      dns_domain = optional(string, "")
      parameters = optional(map(string))
    })))
    nvme_hosts = list(object({
      host_interface = string
    }))
    storage = object({
      svm_name = optional(string)
      api_method = string
    })
  })
}

variable "rancher_config" {
  type = object({
    api_url = string
    token_key = string
    cluster_name = string
    cluster_api_server = string
    kubernetes_version = string
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
