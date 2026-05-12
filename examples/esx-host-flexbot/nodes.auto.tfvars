hosts = {
  esxi01D-lab = {
    blade_spec = {
      dn = "sys/chassis-5/blade-7"
    }
    powerstate = "up"
    installer_image = "images/VMware-VMvisor-Installer-8.0U3e-24677879.x86_64.iso"
    kickstart_template = "templates/ESXi-v8-kickstart.template"
    boot_lun_size = 32
  }
}
