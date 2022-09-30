nodes = {
  masters = {
    k8snode-01 = {
      blade_spec = {
        dn = "sys/chassis-1/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.21"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
    k8snode-02 = {
      blade_spec = {
        dn = "sys/chassis-2/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.22"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
    k8snode-03 = {
      blade_spec = {
        dn = "sys/chassis-3/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.23"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
  }
  workers = {
    k8snode-04 = {
      blade_spec = {
        dn = "sys/chassis-1/blade-2"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.24"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
    k8snode-05 = {
      blade_spec = {
        dn = "sys/chassis-2/blade-2"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.25"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
    k8snode-06 = {
      blade_spec = {
        dn = "sys/chassis-2/blade-2"
        model = "UCSB-B200-M5"
        total_memory = "65536-262144"
      }
      ip = "192.168.1.26"
      powerstate = "up"
      os_image = "ubuntu-20.04.05.01-iboot"
      seed_template = "ubuntu-20.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
      maintenance = {
        execute = false
        synchronized_run = true
        wait_for_node_timeout = 0
        node_grace_timeout = 0
        tasks = ["cordon","drain","restart","uncordon"]
      }
      labels = {}
      taints = []
      force_update = false
    }
  }
}
