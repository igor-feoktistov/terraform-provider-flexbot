provider "flexbot" {
  pass_phrase = var.pass_phrase
  ipam {
    provider = "Infoblox"
    credentials {
      host = var.flexbot_credentials.infoblox.host
      user = var.flexbot_credentials.infoblox.user
      password = var.flexbot_credentials.infoblox.password
      wapi_version = var.node_config.infoblox.wapi_version
      dns_view = var.node_config.infoblox.dns_view
      network_view = var.node_config.infoblox.network_view
    }
    dns_zone = var.node_config.infoblox.dns_zone
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
      api_method = var.node_config.storage.api_method
    }
  }
  rancher_api {
    enabled = true
    provider = "harvester"
    api_url = "https://${var.harvester_config.cluster_vip_addr}"
    token_key = var.harvester_config.harvester_api_token
    insecure = true
    retries = 12
    wait_for_node_timeout = 1800
  }

}

# nodes
resource "flexbot_harvester_node" "node" {
  for_each = var.nodes
  # UCS compute
  compute {
    hostname = each.key
    sp_org = var.node_config.compute.sp_org
    sp_template = var.node_config.compute.sp_template
    blade_spec {
      dn = each.value.blade_spec.dn
    }
    description = "Harvester node ${each.key}"
    powerstate = each.value.powerstate
    safe_removal = false
    wait_for_ssh_timeout = 1800
    ssh_user = var.node_config.compute.ssh_user
    ssh_private_key = var.node_config.compute.ssh_private_key
  }
  # cDOT storage
  storage {
    svm_name = var.node_config.storage.svm_name != null ? var.node_config.storage.svm_name : ""
    bootstrap_lun {
      os_image = each.value.liveiso_image
    }
    boot_lun {
      size = each.value.boot_lun_size
    }
    seed_lun {
      seed_template = each.value.seed_template
    }
  }
  # Compute network
  network {
    # General use interfaces (list)
    dynamic "node" {
      for_each = [for node in var.node_config.network.node: {
        name = node.name
        ip =  node.gateway != "" ? each.value.ip : ""
        subnet = node.subnet
        gateway = node.gateway
        dns_server1 = node.dns_server1
        dns_server2 = node.dns_server2
        dns_server3 = node.dns_server3
        dns_domain = node.dns_domain
      }]
      content {
        name = node.value.name
        ip = node.value.ip
        subnet = node.value.subnet
        gateway = node.value.gateway
        dns_server1 = node.value.dns_server1
        dns_server2 = node.value.dns_server2
        dns_server3 = node.value.dns_server3
        dns_domain = node.value.dns_domain
      }
    }
    # iSCSI initiator networks (list)
    dynamic "iscsi_initiator" {
      for_each = [for iscsi_initiator in var.node_config.network.iscsi_initiator: {
        name = iscsi_initiator.name
        subnet = iscsi_initiator.subnet
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
      }
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    ssh_pub_key = var.node_config.compute.ssh_public_key
    cluster_token = var.harvester_config.cluster_token
    rancher_password = var.harvester_config.rancher_password
    cluster_vip_addr = var.harvester_config.cluster_vip_addr
    node_role = each.value.node_role
  }
}
