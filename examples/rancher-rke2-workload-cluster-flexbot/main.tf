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
    }
  }
  rancher_api {
    enabled = true
    api_url = var.rancher_config.api_url
    token_key = data.flexbot_crypt.rancher_token_key.decrypted
    insecure = true
    retries = 120
    cluster_id = rancher2_cluster_v2.cluster.cluster_v1_id
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

provider "rancher2" {
  api_url = var.rancher_config.api_url
  token_key = data.flexbot_crypt.rancher_token_key.decrypted
  insecure = true
}

resource "rancher2_cloud_credential" "s3" {
  name = "s3"
  description = "S3 bucket access for etcd snapshots"
  s3_credential_config {
    access_key = data.flexbot_crypt.aws_access_key_id.decrypted
    secret_key = data.flexbot_crypt.aws_secret_access_key.decrypted
  }
}

resource "rancher2_cluster_v2" "cluster" {
  name = var.rancher_config.cluster_name
  kubernetes_version = var.rancher_config.kubernetes_version
  enable_network_policy = true
  default_pod_security_policy_template_name = "unrestricted"
  default_cluster_role_for_project_members = "user"
  local_auth_endpoint {
    ca_certs = fileexists("ssl/ca.pem") ? file("ssl/ca.pem") : ""
    enabled = true
    fqdn = "${var.rancher_config.cluster_api_server}:6443"
  }
  rke_config {
    machine_global_config = <<EOF
cni: "cilium"
disable-kube-proxy: true
etcd-expose-metrics: false
cluster-cidr: "172.30.0.0/16"
service-cidr: "172.20.0.0/16"
cluster-dns: "172.20.0.10"
tls-san:
  - ${var.rancher_config.cluster_api_server}
kubelet-arg:
  - max-pods=250
kube-controller-manager-arg:
  - node-cidr-mask-size=23
kube-apiserver-arg:
  - audit-policy-file=/etc/rancher/rke2/audit-policy.yaml
EOF
    machine_selector_config {
      config = {
        "profile" = "cis-1.6"
        "protect-kernel-defaults" = true
      }
    }
    etcd {
      snapshot_retention     = 56
      snapshot_schedule_cron = "0 */3 * * *"
      s3_config {
        bucket                = var.rancher_config.s3_backup.bucket_name
        cloud_credential_name = rancher2_cloud_credential.s3.id
        endpoint              = var.rancher_config.s3_backup.endpoint
        folder                = var.rancher_config.s3_backup.folder
        region                = var.rancher_config.s3_backup.region
        skip_ssl_verify       = true
      }
    }
    upgrade_strategy {
      control_plane_concurrency = "1"
      worker_concurrency        = "1"
      control_plane_drain_options {
        enabled                              = true
        delete_empty_dir_data                = true
        force                                = true
        grace_period                         = 30
        ignore_daemon_sets                   = true
        skip_wait_for_delete_timeout_seconds = 60
        timeout                              = 300
      }
      worker_drain_options {
        enabled                              = false
        delete_empty_dir_data                = true
        force                                = true
        grace_period                         = 30
        ignore_daemon_sets                   = true
        skip_wait_for_delete_timeout_seconds = 60
        timeout                              = 300
      }
    }
    chart_values = <<EOF
rke2-cilium:
  kubeProxyReplacement: strict
  k8sServiceHost: "${var.rancher_config.cluster_api_server}"
  k8sServicePort: 6443
  cni:
    chainingMode: "none"
EOF
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
    svm_name = var.node_config.storage.svm_name != null ? var.node_config.storage.svm_name : ""
    auto_snapshot_on_update = false
    boot_lun {
      os_image = each.value.os_image
      size = each.value.boot_lun_size
    }
    seed_lun {
      seed_template = each.value.seed_template
    }
    data_lun {
      size = each.value.data_lun_size > 0 ? each.value.data_lun_size : 0
    }
    data_nvme {
      size = each.value.data_nvme_size > 0 ? each.value.data_nvme_size : 0
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
    # NVME hosts (list)
    dynamic "nvme_host" {
      for_each = [for nvme_host in var.node_config.nvme_hosts: {
        host_interface = nvme_host.host_interface
      }]
      content {
        host_interface = nvme_host.value.host_interface
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
    ssh_pub_key_ecdsa = var.node_config.compute.ssh_public_key_ecdsa
    rancher_api_url = var.rancher_config.api_url
    node_registration_command = "${rancher2_cluster_v2.cluster.cluster_registration_token[0].node_command} --etcd --controlplane --node-name ${each.key}"
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
    execute = each.value.maintenance.execute
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
    svm_name = var.node_config.storage.svm_name != null ? var.node_config.storage.svm_name : ""
    auto_snapshot_on_update = false
    boot_lun {
      os_image = each.value.os_image
      size = each.value.boot_lun_size
    }
    seed_lun {
      seed_template = each.value.seed_template
    }
    data_lun {
      size = each.value.data_lun_size > 0 ? each.value.data_lun_size : 0
    }
    data_nvme {
      size = each.value.data_nvme_size > 0 ? each.value.data_nvme_size : 0
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
    # NVME hosts (list)
    dynamic "nvme_host" {
      for_each = [for nvme_host in var.node_config.nvme_hosts: {
        host_interface = nvme_host.host_interface
      }]
      content {
        host_interface = nvme_host.value.host_interface
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
    ssh_pub_key_ecdsa = var.node_config.compute.ssh_public_key_ecdsa
    rancher_api_url = var.rancher_config.api_url
    node_registration_command = "${rancher2_cluster_v2.cluster.cluster_registration_token[0].node_command} --worker --node-name ${each.key}"
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
    execute = each.value.maintenance.execute
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
