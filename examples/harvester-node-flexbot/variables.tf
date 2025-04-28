variable "nodes" {
  type = map(object({
    blade_spec = object({
      dn = string
    })
    ip = string
    powerstate = string
    liveiso_image = string
    seed_template = string
    boot_lun_size = number
    node_role = optional(string, "default")
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
      gateway = optional(string, "")
      dns_server1 = optional(string, "")
      dns_server2 = optional(string, "")
      dns_server3 = optional(string, "")
      dns_domain = optional(string, "")
    })))
    storage = optional(object({
      svm_name = optional(string)
      api_method = optional(string, "rest")
    }),
    {
      api_method = "rest"
    })
  })
}

variable "harvester_config" {
  type = object({
    cluster_name = string
    cluster_token = string
    cluster_vip_addr = string
    rancher_password = string
    harvester_api_token = string
  })
}

variable "pass_phrase" {
  type = string
}
