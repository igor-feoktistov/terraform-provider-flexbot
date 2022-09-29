locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}

provider "flexbot" {
  alias = "crypt"
  pass_phrase = var.pass_phrase
}

data "flexbot_crypt" "aws_access_key_id" {
  provider = flexbot.crypt
  name = "aws_access_key_id"
  encrypted = var.rke_config.s3_backup.access_key_id
}

data "flexbot_crypt" "aws_secret_access_key" {
  provider = flexbot.crypt
  name = "aws_secret_access_key"
  encrypted = var.rke_config.s3_backup.secret_access_key
}

data "flexbot_crypt" "ssh_private_key" {
  provider = flexbot.crypt
  name = "ssh_private_key"
  encrypted = var.node_config.compute.ssh_private_key
}

data "flexbot_crypt" "ssh_public_key" {
  provider = flexbot.crypt
  name = "ssh_public_key"
  encrypted = var.node_config.compute.ssh_public_key
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
      zapi_version = var.node_config.storage.zapi_version
    }
  }
  rancher_api {
    enabled = false
    provider = "rke"
    api_url = var.rke_config.api_url
    server_ca_data = var.rke_config.server_ca_data
    client_cert_data = var.rke_config.client_cert_data
    client_key_data = var.rke_config.client_key_data
    insecure = true
    cluster_id = var.rke_config.cluster_name
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}

# RKE nodes
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
      model = each.value.blade_spec.model
      total_memory = each.value.blade_spec.total_memory
    }
    description = var.rke_config.cluster_name
    powerstate = each.value.powerstate
    safe_removal = false
    wait_for_ssh_timeout = 2400
    ssh_user = var.node_config.compute.ssh_user
    ssh_private_key = data.flexbot_crypt.ssh_private_key.decrypted
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
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
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
    ssh_pub_key = data.flexbot_crypt.ssh_public_key.decrypted
    engine_install_url = "https://releases.rancher.com/install-docker/${var.rke_config.docker_version}.sh"
  }
  # Set node labels
  labels = {
    for key, value in merge
      (
        {
          "kubernetes.io/cluster-name" = var.rke_config.cluster_name,
          "kubernetes.io/os-image" = each.value.os_image
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

# RKE hardening is based on CIS 1.6 Benchmark
resource rke_cluster "cluster" {
  depends_on = [flexbot_server.node]
  cluster_name = var.rke_config.cluster_name
  kubernetes_version = var.rke_config.rke_version
  dynamic "nodes" {
    for_each = [for instance in flexbot_server.node: {
      ip = instance.network[0].node[0].ip
      fqdn = instance.network[0].node[0].fqdn
      hostname = instance.compute[0].hostname
    } if var.nodes[instance.compute[0].hostname].rke_member == true]
    content {
      address = nodes.value.ip
      hostname_override = nodes.value.hostname
      internal_address = nodes.value.ip
      user = var.node_config.compute.ssh_user
      role = ["controlplane", "worker", "etcd"]
      ssh_key = data.flexbot_crypt.ssh_private_key.decrypted
    }
  }
  network {
    plugin = "canal"
  }
  services {
    etcd {
      backup_config {
        enabled = true
        interval_hours = 1
        retention = 720
        s3_backup_config {
          region = var.rke_config.s3_backup.region
          endpoint = var.rke_config.s3_backup.endpoint
          access_key = data.flexbot_crypt.aws_access_key_id.decrypted
          secret_key = data.flexbot_crypt.aws_secret_access_key.decrypted
          bucket_name = var.rke_config.s3_backup.bucket_name
          folder = var.rke_config.s3_backup.folder
        }
      }
      extra_args = {
        election-timeout = "5000"
        heartbeat-interval = "500"
      }
      uid = 52034
      gid = 52034
    }
    kube_api {
      always_pull_images = false
      pod_security_policy = false
      audit_log {
        enabled = true
          configuration {
            format = "json"
            max_age = 30
            max_backup = 30
            max_size = 100
            path = "/var/log/kube-audit/audit-log.json"
            policy = jsonencode(jsondecode(file("manifests/audit-policy.json")))
          }
      }
      event_rate_limit {
        enabled = true
      }
      secrets_encryption_config {
        enabled = true
      }
      service_cluster_ip_range = "172.20.0.0/16"
      service_node_port_range = "30000-32767"
      extra_args = {
        external-hostname = var.rke_config.rancher_server
      }
    }
    kube_controller {
      extra_args = {
        feature-gates = "RotateKubeletServerCertificate=true"
      }
      cluster_cidr = "172.30.0.0/16"
      service_cluster_ip_range = "172.20.0.0/16"
    }
    kubelet {
      extra_args = {
        max-pods = "128"
        feature-gates = "RotateKubeletServerCertificate=true"
        tls-cipher-suites = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256"
        #protect-kernel-defaults = "true"
      }
      generate_serving_certificate = true
      cluster_domain = "cluster.local"
      cluster_dns_server = "172.20.0.10"
    }
  }
  authentication {
    sans = [var.rke_config.rancher_server]
  }
  ingress {
    provider = "nginx"
    network_mode = "hostNetwork"
    http_port = 80
    https_port = 443
  }
  upgrade_strategy {
    drain = false
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
    max_unavailable_controlplane = "1"
    max_unavailable_worker = "1"
  }
  restore {
    restore = false
    snapshot_name = ""
  }
}

resource "local_file" "kubeconfig" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = "${local.output_path}/kubeconfig"
  content = rke_cluster.cluster.kube_config_yaml
}
