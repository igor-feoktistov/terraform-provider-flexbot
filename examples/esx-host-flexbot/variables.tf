variable "hosts" {
  type = map(object({
    blade_spec = object({
      dn = string
    })
    ip = optional(string, "")
    powerstate = string
    installer_image = string
    kickstart_template = string
    boot_lun_size = number
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
      kernel_opt = string
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
    storage = optional(object({
      svm_name = optional(string)
    }), {})
    cloud_args = map(any)
  })
}

variable "pass_phrase" {
  type = string
}
