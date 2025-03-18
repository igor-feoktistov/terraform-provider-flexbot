---
page_title: "harvester_node resource"
---

# harvester_node resource

Provides functionality to build SUSE Harvester node on FlexPod.

## Example Usage

```hcl
resource "flexbot_harvester_node" "harvester-node1" {

  # Required - UCS compute
  compute {
    # Required - node name
    hostname = "harvester-node1"
    # Required - UCS Service Profile (server) is to be created here
    sp_org = "org-root/org-Kubernetes"
    # Required - Reference to Service Profile Template (SPT)
    sp_template = "org-root/org-Harvester/ls-ls-Harvester-01"
    # Optional - Service Profile label
    label = "harvester-node1"
    # Optional - Service Profile description
    description = "Harvester node harvester-node1"
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
    # Optional - By default "destroy" will fail if node has powerstate "on".
    safe_removal = false
    # Optional - Wait for SSH is accessible (seconds)
    wait_for_ssh_timeout = 1800
    # Optional - SSH user name.
    # Should match the user defined in cloud-init, typically rancher.
    ssh_user = "rancher"
    # Optional - SSH private key. Same as above. Can be encrypted (built-in decrypt support).
    ssh_private_key = file("~/.ssh/id_rsa")
  }

  # Required - cDOT storage
  storage {
    # Required - Bootstrap LUN
    bootstrap_lun {
      # Required - LiveISO image name
      os_image = "harvester-v1.4.1-amd64-iboot"
    }
    # Required - Boot LUN
    boot_lun {
      # Required - Boot LUN size, GB
      size = 400
    }
    # Required - Seed LUN for cloud-init
    seed_lun {
      # Required - cloud-init template name (see examples/cloud-init in this project).
      # Remove directory path if template uploaded to cDOT storage template repository.
      seed_template = "../../../cloud-init/harvester-v1.4.1-cloud-init-create.template"
    }
    # Optional - SVM name, required for cluster scope provider credentials (rest only)
    svm_name = "vserver"
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
      #fqdn = "harvester-node1.example.com"
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
  }

  # Optional - Cloud Arguments are user defined key/value pairs to resolve in cloud-init template
  # Values can be encrypted (built-in decrypt support)
  cloud_args = {
    ssh_pub_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAxxxxxxxxxxxxxxxxxxxxxxx"
    cluster_token = "harvester-secret"
    rancher_password = "<password in unix password hash format>"
    cluster_vip_addr = "192.168.1.129"
    node_role = "management"
  }
}
```
