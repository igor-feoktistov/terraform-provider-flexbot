nodes = {
  k8s-node1 = {
    blade_spec = {
      dn = "sys/chassis-1/blade-1"
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    ip = "192.168.2.20"
    powerstate = "up"
    os_image = "ubuntu-20.04.05.01-iboot"
    seed_template = "ubuntu-20.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["restart"]
    }
    rke_member = true
  }
  k8s-node1 = {
    blade_spec = {
      dn = "sys/chassis-2/blade-2"
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    ip = "192.168.2.21"
    powerstate = "up"
    os_image = "ubuntu-20.04.05.01-iboot"
    seed_template = "ubuntu-20.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["restart"]
    }
    rke_member = true
  }
  k8s-node3 = {
    blade_spec = {
      dn = "sys/chassis-3/blade-1"
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    ip = "192.168.2.22"
    powerstate = "up"
    os_image = "ubuntu-20.04.05.01-iboot"
    seed_template = "ubuntu-20.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["restart"]
    }
    rke_member = true
  }
}
