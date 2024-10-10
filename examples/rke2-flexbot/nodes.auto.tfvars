nodes = {
  rke2-node1 = {
    blade_spec = {
      dn = "sys/chassis-1/blade-4"
    }
    ip = "192.168.2.20"
    powerstate = "up"
    os_image = "ubuntu-22.04.05.01-iboot"
    seed_template = "ubuntu-22.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["cordon","drain"]
    }
  }
  rke2-node2 = {
    blade_spec = {
      dn = "sys/chassis-2/blade-1"
    }
    ip = "192.168.2.21"
    powerstate = "up"
    os_image = "ubuntu-22.04.05.01-iboot"
    seed_template = "ubuntu-22.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["cordon","drain"]
    }
  }
  rke2-node3 = {
    blade_spec = {
      dn = "sys/chassis-3/blade-1"
    }
    ip = "192.168.2.22"
    powerstate = "up"
    os_image = "ubuntu-22.04.05.01-iboot"
    seed_template = "ubuntu-22.04.05.01-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
    labels = {}
    taints = []
    maintenance = {
      execute = false
      synchronized_run = false
      tasks = ["cordon","drain"]
    }
  }
}
