variable "hosts" {
  type = map(object({
    blade_spec = object({
      dn = string
    })
    ip = string
    powerstate = string
    installer_image = string
    kickstart_template = string
    boot_lun_size = number
    force_update = optional(bool, false)
  }))
}

variable "flexbot_credentials" {
  type = map(object({
    host = string
    user = string
    password = string
  }))
}

variable "host_config" {
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
      ssh_user_password = string
      ssh_public_key = string
      ssh_private_key = string
    })
    network = map(list(object({
      name = string
      subnet = string
      ip_range = optional(string, "")
      gateway = optional(string, "")
      dns_server1 = optional(string, "")
      dns_server2 = optional(string, "")
      dns_server3 = optional(string, "")
      dns_domain = optional(string, "")
      parameters = optional(map(string))
    })))
    storage = object({
      svm_name = optional(string)
      api_method = string
    })
  })
}

variable "vmware_config" {
  type = object({
    host_sdk_user = string
    host_sdk_user_password = string
  })
}

variable "pass_phrase" {
  type = string
}
