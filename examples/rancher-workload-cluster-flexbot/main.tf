locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}

provider "flexbot" {
  alias = "crypt"
  pass_phrase = var.pass_phrase
}

data "flexbot_crypt" "rancher_token_key" {
  provider = flexbot.crypt
  name = "rancher_token_key"
  encrypted = var.rancher_config.token_key
}

data "flexbot_crypt" "aws_access_key_id" {
  provider = flexbot.crypt
  name = "aws_access_key_id"
  encrypted = var.rancher_config.s3_backup.access_key_id
}

data "flexbot_crypt" "aws_secret_access_key" {
  provider = flexbot.crypt
  name = "aws_secret_access_key"
  encrypted = var.rancher_config.s3_backup.secret_access_key
}

provider "rancher2" {
  api_url = var.rancher_config.api_url
  token_key = data.flexbot_crypt.rancher_token_key.decrypted
  insecure = true
}

data "rancher2_setting" "engine_install_url" {
  name = "engine-install-url"
}

# RKE hardening is based on CIS 1.6 Benchmark
resource "rancher2_cluster" "cluster" {
  name = var.rancher_config.cluster_name
/*
  cluster_auth_endpoint {
    enabled = true
    fqdn = "${var.rancher_config.cluster_api_server}:6443"
    ca_certs = fileexists("${local.output_path}/ca_cert") ? base64decode(file("${local.output_path}/ca_cert")) : ""
  }
*/
  enable_network_policy = true
  rke_config {
    kubernetes_version = var.rancher_config.kubernetes_version
    network {
      plugin = "canal"
    }
    services {
      etcd {
        backup_config {
          enabled = true
          interval_hours = 1
          retention = 336
          s3_backup_config {
            region = var.rancher_config.s3_backup.region
            endpoint = var.rancher_config.s3_backup.endpoint
            access_key = data.flexbot_crypt.aws_access_key_id.decrypted
            secret_key = data.flexbot_crypt.aws_secret_access_key.decrypted
            bucket_name = var.rancher_config.s3_backup.bucket_name
            folder = var.rancher_config.s3_backup.folder
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
      }
      kube_controller {
        extra_args = {
          node-cidr-mask-size = "23"
          feature-gates = "RotateKubeletServerCertificate=true"
        }
        cluster_cidr = "172.30.0.0/16"
        service_cluster_ip_range = "172.20.0.0/16"
      }
      kubelet {
        extra_args = {
          max-pods = "500"
          feature-gates = "RotateKubeletServerCertificate=true"
          tls-cipher-suites = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256"
        }
        generate_serving_certificate = true
        cluster_domain = "cluster.local"
        cluster_dns_server = "172.20.0.10"
      }
      scheduler {
        extra_args = {
          config = "/etc/kubernetes/scheduler-config.yaml"
        }
      }
    }
    authentication {
      sans = [var.rancher_config.cluster_api_server]
    }
    ingress {
      provider = "none"
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
  }
}

resource "rancher2_setting" "kubeconfig-generate-token" {
  name = "kubeconfig-generate-token"
  value = false
}

resource "rancher2_setting" "kubeconfig-token-ttl-minutes" {
  depends_on = [rancher2_setting.kubeconfig-generate-token]
  name = "kubeconfig-token-ttl-minutes"
  value = 7200
}

resource "rancher2_setting" "auth-user-session-ttl-minutes" {
  name = "auth-user-session-ttl-minutes"
  value = 960
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
    enabled = true
    api_url = var.rancher_config.api_url
    token_key = data.flexbot_crypt.rancher_token_key.decrypted
    insecure = true
    cluster_id = rancher2_cluster.cluster.id
    wait_for_node_timeout = 1800
    node_grace_timeout = 60
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 30
      ignore_daemon_sets = true
      timeout = 300
    }
  }
}

# Master nodes
resource "flexbot_server" "master" {
  provider = flexbot.server
  for_each = var.nodes.masters
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
    description = var.rancher_config.cluster_name
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
    auto_snapshot_on_update = false
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
    force_update = each.value.force_update
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
        parameters = node.parameters
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
        parameters = node.value.parameters
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
    engine_install_url = data.rancher2_setting.engine_install_url.value
    node_registration_command = "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --etcd --controlplane"
  }
  # Set node labels
  labels = {
    for key, value in merge
      (
        {
          "kubernetes.io/cluster-name" = var.rancher_config.cluster_name,
          "kubernetes.io/os-image" = each.value.os_image
        },
        each.value.labels
      ): key => value
  }
  # Maintenance tasks
  maintenance {
      execute =  each.value.maintenance.execute
      synchronized_run = each.value.maintenance.synchronized_run
      wait_for_node_timeout = each.value.maintenance.wait_for_node_timeout
      node_grace_timeout = each.value.maintenance.node_grace_timeout
      tasks = each.value.maintenance.tasks
  }
  # Restore from snapshot
  restore {
    restore = each.value.restore.restore
    snapshot_name = each.value.restore.snapshot_name
  }
}

# Worker nodes
resource "flexbot_server" "worker" {
  provider = flexbot.server
  for_each = var.nodes.workers
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
    description = var.rancher_config.cluster_name
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
    auto_snapshot_on_update = false
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
    force_update = each.value.force_update
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
        parameters = node.parameters
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
        parameters = node.value.parameters
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
    engine_install_url = data.rancher2_setting.engine_install_url.value
    node_registration_command = "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --worker"
  }
  # Set node labels
  labels = {
    for key, value in merge
      (
        {
          "kubernetes.io/cluster-name" = var.rancher_config.cluster_name,
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
  # Maintenance tasks
  maintenance {
      execute =  each.value.maintenance.execute
      synchronized_run = each.value.maintenance.synchronized_run
      wait_for_node_timeout = each.value.maintenance.wait_for_node_timeout
      node_grace_timeout = each.value.maintenance.node_grace_timeout
      tasks = each.value.maintenance.tasks
  }
  # Restore from snapshot
  restore {
    restore = each.value.restore.restore
    snapshot_name = each.value.restore.snapshot_name
  }
}

/*
resource "local_file" "kubeconfig" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = format("${local.output_path}/kubeconfig")
  content  = rancher2_cluster.cluster.kube_config
}
*/

/*
resource "local_file" "ca_cert" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = format("${local.output_path}/ca_cert")
  content  = rancher2_cluster.cluster.ca_cert
}
*/
