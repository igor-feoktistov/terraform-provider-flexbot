pass_phrase = "secret"

nodes = {
  node-k8s01 = {
    blade_spec = {
      #dn = "sys/chassis-4/blade-7"
      dn = ""
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    powerstate = "up"
    os_image = "ubuntu-18.04.05.02-iboot"
    seed_template = "ubuntu-18.04.05.02-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 128
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
  }
  node-k8s02 = {
    blade_spec = {
      #dn = "sys/chassis-5/blade-1"
      dn = ""
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    powerstate = "up"
    os_image = "ubuntu-18.04.05.02-iboot"
    seed_template = "ubuntu-18.04.05.02-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 128
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
  }
  node-k8s03 = {
    blade_spec = {
      #dn = "sys/chassis-6/blade-3"
      dn = ""
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    powerstate = "up"
    os_image = "ubuntu-18.04.05.02-iboot"
    seed_template = "ubuntu-18.04.05.02-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 128
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
  }
}

flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:jqdbcMI8dI5Dq<...skip...>yoskcRz9UUP+gN4v0Eo="
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:kEqDbvk/DwABc<...skip...>orS6UIjo21DpA6QTFDOc="
  }
  cdot = {
    host = "vserver.example.com"
    user = "vsadmin"
    password = "base64:qiZIN5H04oK15<...skip...>7k4uoBIIg/boi2n3+4kQ="
  }
}

node_config = {
  infoblox = {
    wapi_version = "2.5"
    dns_view = "Internal"
    network_view = "default"
    dns_zone = "example.com"
  }
  compute = {
    sp_org = "org-root/org-Kubernetes"
    sp_template = "org-root/org-Kubernetes/ls-K8S-01-SPD-0"
    ssh_user = "cloud-user"
    ssh_public_key_path = "~/.ssh/id_rsa.pub"
    ssh_private_key_path = "~/.ssh/id_rsa"
  }
  network = {
    node = [
      {
        name = "eth2"
        subnet = "192.168.1.0/24"
        gateway = "192.168.1.1"
        dns_server1 = "192.168.1.10"
        dns_server2 = ""
        dns_domain = "example.com"
      }
    ]
    iscsi_initiator = [
      {
        name = "iscsi0"
        subnet = "192.168.2.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_domain = ""
      },
      {
        name = "iscsi1"
        subnet = "192.168.3.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_domain = ""
      }
    ]
  }
  storage = {
    zapi_version = ""
  }
}
