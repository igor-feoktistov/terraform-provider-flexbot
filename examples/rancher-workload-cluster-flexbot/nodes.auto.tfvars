nodes = {
  masters = {
    k8snode-01 = {
      blade_spec = {
        dn = "sys/chassis-5/blade-4"
        model = "UCSB-B200-M5"
        total_memory = "65536"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
    k8snode-02 = {
      blade_spec = {
        dn = "sys/chassis-4/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "65536"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
    k8snode-03 = {
      blade_spec = {
        dn = "sys/chassis-6/blade-3"
        model = "UCSB-B200-M5"
        total_memory = "65536"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 64
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
  }
  workers = {
    k8snode-04 = {
      blade_spec = {
        dn = "sys/chassis-7/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "262144"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 128
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
    k8snode-05 = {
      blade_spec = {
        dn = "sys/chassis-7/blade-2"
        model = "UCSB-B200-M5"
        total_memory = "262144"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 128
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
    k8snode-06 = {
      blade_spec = {
        dn = "sys/chassis-5/blade-1"
        model = "UCSB-B200-M5"
        total_memory = "262144"
      }
      powerstate = "up"
      os_image = "ubuntu-18.04.05.01-iboot"
      seed_template = "ubuntu-18.04.05.01-cloud-init.template"
      boot_lun_size = 32
      data_lun_size = 128
      restore = {restore = false, snapshot_name = ""}
      snapshots = []
    }
  }
}
