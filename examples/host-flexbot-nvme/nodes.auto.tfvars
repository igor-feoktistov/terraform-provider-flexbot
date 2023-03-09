nodes = {
  node-k8s01 = {
    blade_spec = {
      dn = "sys/chassis-5/blade-4"
      model = "UCSB-B200-M5"
      total_memory = "65536-262144"
    }
    powerstate = "up"
    os_image = "ubuntu-22.04.02.01-iboot"
    seed_template = "ubuntu-22.04.02.01-cloud-init.template"
    boot_lun_size = 32
    data_nvme_size = 32
    restore = {restore = false, snapshot_name = ""}
    snapshots = []
  }
}
