locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}

provider "flexbot" {
  alias = "server"
  pass_phrase = var.pass_phrase
  synchronized_updates = true
  ipam {
    provider = "Infoblox"
    credentials {
      host = var.flexbot_credentials.infoblox.host
      user = var.flexbot_credentials.infoblox.user
      password = var.flexbot_credentials.infoblox.password
      wapi_version = var.node_config.infoblox.wapi_version
      dns_view = var.node_config.infoblox.dns_view
      network_view = var.node_config.infoblox.network_view
      ext_attributes = {
        "Region" = "us-east-1"
        "Site" = "onprem-us-east-1-01"
      }
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
    provider = "rke2"
    api_url = "https://${var.rke2_config.rke2_server}:6443"
    server_ca_data = var.rke2_config.server_ca_data
    client_cert_data = var.rke2_config.client_cert_data
    client_key_data = var.rke2_config.client_key_data
    insecure = true
    cluster_name = var.rke2_config.rke2_cluster
    cluster_id = var.rke2_config.rke2_cluster
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}

# RKE2 nodes
resource "flexbot_server" "node" {
  provider = flexbot.server
  for_each = var.nodes
  # UCS compute
  compute {
    hostname = each.key
    sp_org = var.node_config.compute.sp_org
    sp_template = var.node_config.compute.sp_template
    blade_spec {
      dn = each.value.blade_spec.dn
    }
    description = var.rke2_config.rke2_cluster
    powerstate = each.value.powerstate
    safe_removal = false
    wait_for_ssh_timeout = 2400
    ssh_user = var.node_config.compute.ssh_user
    ssh_private_key = var.node_config.compute.ssh_private_key
    ssh_node_init_commands = ["sudo cloud-init status --wait || true"]
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
  }
  # cDOT storage
  storage {
    auto_snapshot_on_update = true
    boot_lun {
      os_image = each.value.os_image
      size = each.value.boot_lun_size
    }
    seed_lun {
      seed_template = each.value.seed_template
    }
    data_lun {
      size = each.value.data_lun_size
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
        parameters = iscsi_initiator.parameters
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
        parameters = iscsi_initiator.value.parameters
      }
    }
  }
  # Maintenance tasks
  maintenance {
      execute =  each.value.maintenance.execute
      synchronized_run = each.value.maintenance.synchronized_run
      tasks = each.value.maintenance.tasks
  }
  # Storage snapshots
  dynamic "snapshot" {
    for_each = [for snapshot in each.value.snapshots: {
      name = snapshot.name
      fsfreeze = snapshot.fsfreeze
    }]
    content {
      name = snapshot.value.name
      fsfreeze = snapshot.value.fsfreeze
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    cloud_user = var.node_config.compute.ssh_user
    ssh_pub_key = var.node_config.compute.ssh_public_key
    rke2_version = var.rke2_config.rke2_version
    rke2_token = var.rke2_config.rke2_token
    rke2_server = var.rke2_config.rke2_server
    s3_endpoint = var.rke2_config.s3_backup.endpoint
    s3_region = var.rke2_config.s3_backup.region
    s3_bucket = var.rke2_config.s3_backup.bucket
    s3_folder = var.rke2_config.s3_backup.folder
    s3_access_key_id = var.rke2_config.s3_backup.access_key_id
    s3_secret_access_key = var.rke2_config.s3_backup.secret_access_key
  }
  # Set node labels
  labels = {
    for key, value in merge
      (
        {
          "kubernetes.io/cluster-name" = var.rke2_config.rke2_cluster,
          "kubernetes.io/os-image" = each.value.os_image,
          "node-role.kubernetes.io/worker" = "true"
        },
        each.value.labels
      ): key => value
  }
  # Set node taints
  dynamic "taints" {
    for_each = each.value.taints
    content {
      key = taints.value["key"]
      value = taints.value["value"]
      effect = taints.value["effect"]
    }
  }
  # Restore from snapshot
  restore {
    restore = each.value.restore.restore
    snapshot_name = each.value.restore.snapshot_name
  }
}
