variable "nodes" {
  type = map(object({
    hosts = list(string)
    compute_blade_spec_dn = list(string)
    compute_blade_spec_model = string
    compute_blade_spec_total_memory = string
    os_image = string
    seed_template = string
    boot_lun_size = number
    data_lun_size = number
  }))
}

variable "flexbot_credentials" {
  type = map(object({
    host = string
    user = string
    password = string
  }))
}

variable "infoblox_config" {
  type = map
}

variable "node_compute_config" {
  type = map
}

variable "node_network_config" {
  type = map(list(object({
    name = string
    subnet = string
    gateway = string
    dns_server1 = string
    dns_server2 = string
    dns_domain = string
  })))
}

variable "snapshots" {
  type = list(object({
    name = string
    fsfreeze = bool
  }))
  default = []
}

variable "zapi_version" {
  type = string
  description = "cDOT ZAPI version"
  default = ""
}

variable "rancher_config" {
  type = map
}

variable "kubernetes_version" {
  type = string
  description = "RKE Kubernetes version"
  default = "v1.18.9-rancher1-1"
}

variable "pass_phrase" {
  type = string
}

variable "token_key" {
  type = string
}

variable "output_path" {
  type = string
  description = "Path to output directory"
}
