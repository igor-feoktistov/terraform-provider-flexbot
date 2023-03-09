flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:8EJswfF<reducted>JF4877DCdLg="
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:kEqDbvk/DwABc<reducted>orS6UIjo21DpA6QTFDOc="
  }
  cdot = {
    host = "vserver.example.com"
    user = "vsadmin"
    password = "base64:qiZIN5H04oK15<reducted>7k4uoBIIg/boi2n3+4kQ="
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
    sp_template = "org-root/org-Kubernetes/ls-K8S-01"
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
  nvme_hosts = [
    {
      host_interface = "iscsi0"
    },
    {
      host_interface = "iscsi1"
    }
  ]
  storage = {
    api_method = "rest"
  }
}
