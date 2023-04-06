provider "flexbot" {

  # Optional - password phrase to decrypt user names and passwords in credentials (if encrypted).
  # Machine ID is used by default.
  # Values for user and password in credentials can be encrypted.
  # See "flexbot_crypt" datasource example on how to generate encrypted user / password values.
  pass_phrase = "secret"

  # Optional - environment variable to pass encryption key to decrypt "pass_phrase" (if encrypted).
  # If "pass_phrase" is encrypted, machine ID is used as default password phrase unless "pass_phrase_env_key" is defined.
  pass_phrase_env_key = "PASS_PHRASE_ENC_KEY"

  # Required - IPAM is implemented via pluggable providers.
  # Only "Infoblox" and "Internal" providers are supported at this time.
  # "Internal" provider expects you to supply "ip" and "fqdn" in network configuration.
  # Define 'provider = "Internal"' if you manage IPAM via terraform provider.
  ipam {
    provider = "Infoblox"
    # Required - Credentials for Infoblox master
    credentials {
      host = "ib.example.com"
      user = "admin"
      password = "secret"
      wapi_version = "2.5"
      dns_view = "Internal"
      network_view = "default"
    }
    # Required - Compute node FQDN is <hostname>.<dns_zone>
    dns_zone = "example.com"
  }
  # Example with "Internal" IPAM provider
  #ipam {
    #provider = "Internal"
  #}

  # Required - UCS compute
  compute {
    # Required - Credentials for UCSM
    credentials {
      host = "ucsm.example.com"
      user = "admin"
      password = "secret"
    }
  }

  # Required - cDOT storage
  storage {
    # Required - Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended.
    credentials {
      host = "svm.example.com"
      user = "vsadmin"
      password = "secret"
      # Optional - ZAPI version to handle older OnTap
      zapi_version = "1.160"
    }
  }

  # This example is for rancher2 API provider
  # Optional - Rancher API
  # Rancher API helps with node management in Kubernetes cluster:
  #  - graceful node removal (cordon/drain);
  #  - graceful node blade specs updates (cordon/drain/uncordon);
  #  - graceful node image/cloud-init updates (cordon/drain/uncordon).
  #  - maintain labels and taints
  rancher_api {
    # Optional (default is false)
    enabled = true
    # Optional (default is "rancher2")
    provider = "rancher2"
    api_url = "https://rancher.example.com"
    token_key = "token-xxx"
    insecure = true
    retries = 6
    cluster_id = rancher2_cluster.cluster.id
    # Optional - Grace timeout in seconds after each node update in changing
    #            blade_spec or os_image/seed_template. Checks for node status
    #            "active" during timeout.
    node_grace_timeout = 60
    # Optional - Wait timeout for node status "active". Assigned compute blade
    #            specs will be recorded in node annotations if timeout > 0.
    wait_for_node_timeout = 1800
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 60
      ignore_daemon_sets = true
      timeout = 1800
    }
  }
  #
  # This example is for RKE or any other Kubernetes API provider
  #
  #rancher_api {
    #enabled = true
    #provider = "rke"
    #api_url = "https://rke.example.com:6443"
    #cluster_id = "onprem-us-east-1-01"
    #server_ca_data = "LS0tLS1CEUdJUiBDRVJUSKZJQ0F<...reducted...>tLSItYQo="
    #client_cert_data = "LS0dLS1TRUdJTi<...reducted...>BFEUaLS0sLQo="
    #client_key_data = "base64:giZIN7H04oQw5<...reducted...>8k4uoWEIg/woi2n3+4kQ="
    #drain_input {
    #  force = true
    #  delete_local_data = true
    #  grace_period = 60
    #  ignore_daemon_sets = true
    #  timeout = 1800
    #}
  #}

  # Optional - Synchronized nodes updates.
  # It is highly suggested to enable it when Rancher API is enabled.
  # Affects the following tasks when enabled:
  #  - node blade specs updates;
  #  - node image/cloud-init updates.
  # Enforces sequential nodes updates.
  # Failure in any node update task would stop updates for all others nodes.
  synchronized_updates = true

}

resource "flexbot_server" "k8s-node1" {

  # Required - UCS compute
  compute {
    # Required - node name
    hostname = "k8s-node1"
    # Required - UCS Service Profile (server) is to be created here
    sp_org = "org-root/org-Kubernetes"
    # Required - Reference to Service Profile Template (SPT)
    sp_template = "org-root/org-Kubernetes/ls-K8S-SubProd-01"
    # Optional - Service Profile label
    label = "worker"
    # Optional - Service Profile description
    description = "worker, cluster us-west-dc12-01"
    # Optional - Blade spec to find blade (all specs are optional)
    # Optional - Blade spec to find blade (all specs are optional)
    blade_spec {
      # Blade Dn, supports regexp
      #dn = "sys/chassis-4/blade-3"
      #dn = "sys/chassis-9/blade-[0-9]+"
      # Blade model, supports regexp
      model = "UCSB-B200-M3"
      #model = "UCSB-B200-M[45]"
      # Number of CPUs, supports range
      #num_of_cpus = "2"
      # Number of cores, supports range
      #num_of_cores = "36"
      #num_of_cores = "24-36"
      # Number of threads, supports range
      #num_of_threads = "72"
      #num_of_threads = "48-72"
      # Total memory in MB, supports range
      total_memory = "65536-262144"
    }
    # Optional - Blade powerstate management.
    # Default is "up".
    # Would try to execute graceful shutdown for "down" state following HW shutdown after 60s timeout.
    powerstate = "up"
    # Optional - By default "destroy" will fail if server has powerstate "on".
    # It is highly recommended to disable it for Rancher nodes to prevent failures
    # in automated nodes management.
    safe_removal = false
    # Optional - Wait for SSH is accessible (seconds), default is 0 (no wait)
    wait_for_ssh_timeout = 1800
    # Optional - SSH user name.
    # Required for consistent snapshosts and if defined "ssh_node_init_commands".
    # Should match the user defined in cloud-init for image access.
    ssh_user = "cloud-user"
    # Optional - SSH private key. Same as above. Can be encrypted (built-in decrypt support).
    ssh_private_key = file("~/.ssh/id_rsa")
    # Optional - Brings "provisioner" functionality inside "flexbot_server" resource for better
    # error handling and node updates functionality.
    ssh_node_init_commands = [
      "sudo cloud-init status --wait || true",
      "curl ${data.rancher2_setting.docker_install_url.value} | sh",
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --etcd --controlplane"
    ]
    # Optional - commands to re-size boot disk on host
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    # Optional - commands to re-size data disk on host
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
  }

  # Required - cDOT storage
  storage {
    # Required - Boot LUN
    boot_lun {
      # Required - Boot LUN size, GB
      size = 20
      # Required - OS image name
      os_image = "rhel-7.7.01-iboot"
    }
    # Required - Seed LUN for cloud-init
    seed_lun {
      # Required - cloud-init template name (see examples/cloud-init in this project).
      # Remove directory path if template uploaded to cDOT storage template repository.
      seed_template = "../../../examples/cloud-init/rhel7-cloud-init.template"
    }
    # Optional - Data LUN
    data_lun {
      # Data LUN size, GB
      size = 50
    }
    # Optional - Data NVME over TCP disk
    data_nvme {
      # Data disk size, GB
      size = 50
    }
    # Optional - SVM name, required for cluster scope provider storage credentials (rest only)
    svm_name = "vserver"
    # Optional - automatically take a snapshot before any image update
    auto_snapshot_on_update = true
    # Optional - force node re-imaging.
    # Make sure to set it back to "false" once completed in order to avoid node re-imaging on next apply.
    force_update = true
  }

  # Required - Compute network
  network {
    # Required - Generic network (multiple nodes are allowed)
    node {
      # Required - Name should match respective vNIC name in Service Profile Template
      name = "eth2"
      # Optional - if IP needs to be assigned statically, not allocated by IPAM from subnet or IP range
      #ip = "192.168.1.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1.example.com"
      # IPAM allocates IP for node interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.1.0/24"
      # Optional - IP range to allocate IP from
      # IPAM allocates IP from subnet if "ip_range" is not specified
      # For Infoblox plugin it should match "Start-End" IP's of IPv4 Reserved Range
      #ip_range = "192.168.1.32-192.168.1.64"
      # Required - default GW IP address
      gateway = "192.168.1.1"
      # Optional - Arguments for node resolver configuration.
      dns_server1 = "192.168.1.10"
      dns_server2 = "192.168.4.10"
      dns_server3 = "192.168.5.10"
      dns_domain = "example.com"
      # Optional - Parameters are user defined key/value pairs to resolve in cloud-init template network interface settings
      parameters = {
        mtu = "9000"
      }
    }
    # Required - iSCSI initiator network #1
    iscsi_initiator {
      # Required - Name should match respective iSCSI vNIC name in Service Profile Template
      name = "iscsi0"
      # Optional - if IP needs to be assigned statically, not allocated by IPAM from subnet or IP range
      #ip = "192.168.2.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1-i1.example.com"
      # IPAM allocates IP for iSCSI interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.2.0/24"
      # Optional - IP range to allocate IP from
      # IPAM allocates IP from subnet if "ip_range" is not specified
      # For Infoblox plugin it should match "Start-End" IP's of IPv4 Reserved Range
      #ip_range = "192.168.2.32-192.168.2.64"
    }
    # Optional, but highly suggested - iSCSI initiator network #2
    iscsi_initiator {
      # required - Name should match respective iSCSI vNIC name in Service Profile Template
      name = "iscsi1"
      # Optional - if IP needs to be assigned statically, not allocated by IPAM from subnet or IP range
      #ip = "192.168.3.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1-i2.example.com"
      # IPAM allocates IP for iSCSI interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.3.0/24"
      # Optional - IP range to allocate IP from
      # IPAM allocates IP from subnet if "ip_range" is not specified
      # For Infoblox plugin it should match "Start-End" IP's of IPv4 Reserved Range
      #ip_range = "192.168.3.32-192.168.3.64"
    }
    # Optional (required if "data_nvme" is defined), support for NVME over TCP
    nvme_host {
      # required, should be either node or iscsi_initiator name.
      # nvme host will use specified interface for discovery/connection
      # in this example NVME host will share interface with iSCSI initiator
      host_interface = "iscsi0"
    }
    # Optional, but two hosts are highly suggested for multipath
    nvme_host {
      host_interface = "iscsi1"
    }
  }

  # Optional - cDOT storage snapshots to take while server lifecycle management
  # You may want to keep the list empty until server is built
  snapshot {
      # Required - Snapshot name
      name = "terraform-2020-07-24-17:15"
      # Optional - Ensures "fsfreeze" for every filesystem on iSCSI LUN's before taking snapshot.
      # Requires ssh_user and ssh_private_key parameters in "compute"
      fsfreeze = true
  }

  # Optional - Cloud Arguments are user defined key/value pairs to resolve in cloud-init template
  # Values can be encrypted (built-in decrypt support)
  cloud_args = {
    cloud_user = "cloud-user"
    ssh_pub_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAxxxxxxxxxxxxxxxxxxxxxxx"
  }

  # Optional - Kubernetes node labels are user defined key/value pairs - requires Rancher API enabled
  labels = {
    "kubernetes.io/cluster-name" = "us-west-flexpod-01"
  }

  # Optional - Kubernetes node taints - requires Rancher API enabled
  taints {
    key = "app"
    value = "MyApp"
    effect = "NoSchedule"
  }
  taints {
    key = "app"
    value = "MyApp"
    effect = "NoExecute"
  }

  # Restore from snapshot
  # Optional - restore server LUN's from snapshot.
  restore {
    # Make sure to set "restore=false" once it's completed.
    restore = true
    # Optional - by default it finds latest snapshot created by the provider
    #            if you set auto_snapshot_on_update to true for storage.
    # List of all available snapshots you can find in state file (look for snapshosts[]) for the resource.
    snapshot_name = "k8s-node1.snap.1"
  }

  # Maintenance tasks
  # Optional - execute list of maintenance tasks in defined sequence
  maintenance {
    # Make sure to set "execute=false" once it's completed.
    execute = true
    # Run one node at time, skip execution on other nodes if current node task failed
    synchronized_run = true
    # Number of seconds to wait for node state "active" after restart task is completed and node is available in network
    # Requires Rancher API enabled in provider
    wait_for_node_timeout = 0
    # Number of seconds to wait after restart is completed or node state change to "active" (see above)
    node_grace_timeout = 0
    # List of tasks to execute sequentially
    # The following tasks are supported: cordon, uncordon, drain, restart
    # The tasks cordon, uncordon, and drain require Rancher API enabled in provider
    tasks = ["cordon","drain","restart","uncordon"]
  }

  # Optional - Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = "cloud-user"
    private_key = file("~/.ssh/id_rsa")
    timeout = "10m"
  }

  # Optional - Provisioner to initialize node
  # Same functionality is available via "ssh_node_init_commands" parameter
  # in "compute". For Rancher nodes it is highly suggested to define node
  # initialization via "ssh_node_init_commands" parameter.
  provisioner "remote-exec" {
    inline = [
      "curl https://releases.rancher.com/install-docker/19.03.sh | sh",
    ]
  }
}

# Show server IP address
output "ip_address" {
  value = flexbot_server.k8s-node1.network[0].node[0].ip
}

# Show server FQDN
output "fqdn" {
  value = flexbot_server.k8s-node1.network[0].node[0].fqdn
}
