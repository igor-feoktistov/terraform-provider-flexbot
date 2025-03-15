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

node_config = {
  infoblox = {
    wapi_version = "2.5"
    dns_view = "Internal"
    network_view = "default"
    dns_zone = "example.com"
  }
  compute = {
    sp_org = "org-root/org-Harvester"
    sp_template = "org-root/org-Harvester/ls-Harvester"
    ssh_user = "rancher"
    ssh_public_key = "base64:KINnLzalKy<reducted>"
    ssh_private_key = "base64:3KzGZkFijM<reducted>"
  }
  network = {
    node = [
      {
        name = "eth2"
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
}

cluster_config = {
  cluster_token = "base64:sd6qWrbV7tdHl<reducted>"
  cluster_vip_addr = "192.168.1.129"
  rancher_password = "base64:zp9qVpaS9udHi<reducted>"
}
