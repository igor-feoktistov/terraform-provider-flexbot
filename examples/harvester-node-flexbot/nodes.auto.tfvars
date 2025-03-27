nodes = {
  harv-node01 = {
    blade_spec = {
      dn = "sys/chassis-1/blade-2"
    }
    ip = "192.168.1.20"
    powerstate = "up"
    liveiso_image = "harvester-v1.4.1-amd64-iboot"
    seed_template = "harvester-v1.4.1-cloud-init-create.template"
    boot_lun_size = 400
    node_role = "management"
  }
  harv-node01 = {
    blade_spec = {
      dn = "sys/chassis-2/blade-2"
    }
    ip = "192.168.1.21"
    powerstate = "up"
    liveiso_image = "harvester-v1.4.1-amd64-iboot"
    seed_template = "harvester-v1.4.1-cloud-init-join.template"
    boot_lun_size = 400
    node_role = "worker"
  }
}
