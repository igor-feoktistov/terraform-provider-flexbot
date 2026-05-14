---
page_title: "esx_host resource"
---

# esx_host resource

Provides functionality to build VMware ESXi hosts on FlexPod.

## Example Usage

```hcl
resource "flexbot_esx_host" "esxi-host1" {

  # Required - UCS compute
  compute {
    # Required - host name
    hostname = "esxi-host1"
    # Required - UCS Service Profile (host) is to be created here
    sp_org = "org-root/org-ESXi"
    # Required - Reference to Service Profile Template (SPT)
    sp_template = "org-root/org-ESXi/ls-ESXi-01"
    # Optional - Service Profile label
    label = "esxi-host1"
    # Optional - Service Profile description
    description = "VMware ESXi host esxi-host1"
    # Optional - Blade spec to find blade (all specs are optional)
    blade_spec {
      # Blade Dn, supports regexp
      #dn = "sys/chassis-4/blade-3"
      #dn = "sys/chassis-9/blade-[0-9]+"
      # Blade model, supports regexp
      model = "UCSB-B200-M5"
      #model = "UCSB-B200-M[56]"
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
    # Would try to execute graceful shutdown for "down" state following HW shutdown.
    powerstate = "up"
    # Optional - By default "destroy" will fail if host has powerstate "on".
    safe_removal = false
    # Optional - Default is "bios" (legacy BIOS boot)
    firmware = "efi"
    # Optional - Default is "ks=file:///ks.cfg"
    kernel_opt = "ks=file:///ks.cfg allowLegacyCPU=true"
  }

  # Required - cDOT storage
  storage {
    # Required - Boot LUN
    boot_lun {
      # Required - VMware ESXi ISO installer location
      #installer_image = "https://repo.example.com:/vmware-images/VMware-VMvisor-Installer-8.0U3e-24677879.x86_64.iso"
      #installer_image = "file:///var/lib/vmware-images/VMware-VMvisor-Installer-8.0U3e-24677879.x86_64.iso"
      installer_image = "images/VMware-VMvisor-Installer-8.0U3e-24677879.x86_64.iso"
      # Required - VMware ESXi kickstart template location
      #kickstart_template = "https://repo.example.com:/vmware-templates/ESXi-v8-kickstart.template"
      #kickstart_template = "file:///var/lib/vmware-templates/vmware-templates/ESXi-v8-kickstart.template"
      kickstart_template = "templates/ESXi-v8-kickstart.template"
      # Required - Boot LUN size, GB
      size = 32
    }
    # Optional - SVM name, required for cluster scope provider credentials (REST only)
    svm_name = "vserver"
  }

  # Required - Compute network
  network {
    # Required - Generic network (multiple nodes are allowed)
    node {
      # Required - Name should match respective vNIC name in Service Profile Template
      name = "vmnic0"
      # Optional - if IP needs to be assigned statically, not allocated by IPAM from subnet or IP range
      #ip = "192.168.1.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "esxi-host1.example.com"
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
      #fqdn = "esxi-host1-i1.example.com"
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
      #fqdn = "esxi-host1-i2.example.com"
      # IPAM allocates IP for iSCSI interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.3.0/24"
      # Optional - IP range to allocate IP from
      # IPAM allocates IP from subnet if "ip_range" is not specified
      # For Infoblox plugin it should match "Start-End" IP's of IPv4 Reserved Range
      #ip_range = "192.168.3.32-192.168.3.64"
    }
  }

  # Optional - Cloud Arguments are user defined key/value pairs to resolve in kickstart template.
  # Values can be encrypted (built-in decrypt support).
  cloud_args = {
    ssh_user = "root"
    ssh_user_password = "<encrypted passsword>"
    ssh_pub_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAxxxxxxxxxxxxxxxxxxxxxxx"
    host_sdk_user = "svc-maintenance"
    host_sdk_user_password = "<encrypted passsword>"
  }
}
```
