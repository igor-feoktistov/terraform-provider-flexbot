provider "flexbot" {
  pass_phrase = var.pass_phrase
  ipam {
    provider = "Infoblox"
    credentials {
      host = var.flexbot_credentials.infoblox.host
      user = var.flexbot_credentials.infoblox.user
      password = var.flexbot_credentials.infoblox.password
      wapi_version = var.host_config.infoblox.wapi_version
      dns_view = var.host_config.infoblox.dns_view
      network_view = var.host_config.infoblox.network_view
    }
    dns_zone = var.host_config.infoblox.dns_zone
  }
  compute {
    credentials {
      host = var.flexbot_credentials.ucsm.host
      user = var.flexbot_credentials.ucsm.user
      password = var.flexbot_credentials.ucsm.password
    }
  }
  storage {
    credentials {
      host = var.flexbot_credentials.cdot.host
      user = var.flexbot_credentials.cdot.user
      password = var.flexbot_credentials.cdot.password
      api_method = var.host_config.storage.api_method
    }
  }
  vmware_api {
    enabled = true
    provider = "host"
    host_sdk_user = var.vmware_config.host_sdk_user
    host_sdk_user_password = var.vmware_config.host_sdk_user_password
    insecure = true
    wait_for_host_installer_timeout = 1800
    wait_for_host_boot_timeout = 600
  }
}

# hosts
resource "flexbot_esx_host" "host" {
  for_each = var.hosts
  # UCS compute
  compute {
    hostname = each.key
    sp_org = var.host_config.compute.sp_org
    sp_template = var.host_config.compute.sp_template
    blade_spec {
      dn = each.value.blade_spec.dn
    }
    firmware = "bios"
    powerstate = each.value.powerstate
    safe_removal = true
    wait_for_ssh_timeout = 1800
    ssh_user = var.host_config.compute.ssh_user
    ssh_private_key = var.host_config.compute.ssh_private_key
  }
  # cDOT storage
  storage {
    svm_name = var.host_config.storage.svm_name != null ? var.host_config.storage.svm_name : ""
    boot_lun {
      installer_image = each.value.installer_image
      kickstart_template = each.value.kickstart_template
      size = each.value.boot_lun_size
    }
  }
  # Compute network
  network {
    # General use interfaces (list)
    dynamic "node" {
      for_each = [for node in var.host_config.network.node: {
        name = node.name
        ip =  node.gateway != "" ? each.value.ip : ""
        subnet = node.subnet
        ip_range = node.ip_range
        gateway = node.gateway
        dns_server1 = node.dns_server1
        dns_server2 = node.dns_server2
        dns_domain = node.dns_domain
      }]
      content {
        name = node.value.name
        ip = node.value.ip
        subnet = node.value.subnet
        ip_range = node.value.ip_range
        gateway = node.value.gateway
        dns_server1 = node.value.dns_server1
        dns_server2 = node.value.dns_server2
        dns_domain = node.value.dns_domain
      }
    }
    # iSCSI initiator networks (list)
    dynamic "iscsi_initiator" {
      for_each = [for iscsi_initiator in var.host_config.network.iscsi_initiator: {
        name = iscsi_initiator.name
        subnet = iscsi_initiator.subnet
        ip_range = iscsi_initiator.ip_range
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
        ip_range = iscsi_initiator.value.ip_range
      }
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    ssh_user = var.host_config.compute.ssh_user
    ssh_user_password = var.host_config.compute.ssh_user_password
    ssh_pub_key = var.host_config.compute.ssh_public_key
    host_sdk_user = var.vmware_config.host_sdk_user
    host_sdk_user_password = var.vmware_config.host_sdk_user_password
  }
}
