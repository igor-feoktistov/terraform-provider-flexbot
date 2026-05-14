flexbot_credentials = {
  infoblox = {
    host = "192.168.1.10"
    user = "admin"
    password = "base64:5EJ<reducted>"
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:37IN<reducted>"
  }
  cdot = {
    host = "svm.example.com"
    user = "vsadmin"
    password = "base64:9WBf<reducted>"
  }
}

host_config = {
  infoblox = {
    wapi_version = "2.5"
    dns_view = "Internal"
    network_view = "default"
    dns_zone = "example.com"
  }
  compute = {
    sp_org = "org-root/org-ESXi"
    sp_template = "org-root/org-ESXi/ls-ESXi-template"
    kernel_opt = "ks=file:///ks.cfg allowLegacyCPU=true"
  }
  network = {
    node = [
      {
        name = "vmnic0"
        subnet = "192.168.1.0/24"
        gateway = "192.168.1.1"
        dns_server1 = "192.168.1.10"
        dns_server2 = "192.168.2.10"
        dns_server3 = "192.168.3.10"
        dns_domain = "example.com"
      }
    ]
    iscsi_initiator = [
      {
        name = "iscsi0"
        subnet = "192.168.2.0/24"
      },
      {
        name = "iscsi1"
        subnet = "192.168.3.0/24"
      }
    ]
  }
  cloud_args = {
    ssh_user = "root"
    ssh_user_password = "base64:sTDJKRHGF<reducted>"
    ssh_public_key = "base64:RtFnLs<reducted>"
    host_sdk_user = "svc-maintenance"
    host_sdk_user_password = "base64:dSTfHGF9<reducted>"
  }
}
