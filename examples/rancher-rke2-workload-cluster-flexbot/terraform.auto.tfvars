flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:<reducted>"
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:<reducted>"
  }
  cdot = {
    host = "svm.example.com"
    user = "vsadmin"
    password = "base64:<reducted>"
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
    sp_template = "org-root/org-Kubernetes/ls-K8S-SPT01"
    ssh_user = "cloud-user"
    ssh_public_key = "base64:<reducted>"
    ssh_private_key = "base64:<reducted>"
    ssh_public_key_ecdsa = "base64:<reducted>"
    ssh_private_key_ecdsa = "base64:<reducted>"
  }
  network = {
    node = [
      {
        name = "eth2"
        subnet = "192.168.1.0/24"
        gateway = "192.168.1.1"
        dns_server1 = "192.168.1.10"
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = "example.com"
        parameters = {}
      },
      {
        name = "eth3"
        subnet = "192.168.2.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = ""
        parameters = {
          mtu = "9000"
        }
      }
    ]
    iscsi_initiator = [
      {
        name = "iscsi0"
        subnet = "192.168.3.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = ""
        parameters = {}
      },
      {
        name = "iscsi1"
        subnet = "192.168.4.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = ""
        parameters = {}
      }
    ]
  }
  storage = {
    api_method = "rest"
  }
}

rancher_config = {
  api_url = "https://rancher.example.com"
  cluster_name = "op-us-west-01-01"
  cluster_api_server = "op-us-west-01-01.rancher.example.com"
  token_key = "base64:<reducted>"
  kubernetes_version = "v1.24.9+rke2r2"
  s3_backup = {
    region = "us-east-1"
    endpoint = "s3-accesspoint.us-east-1.amazonaws.com"
    access_key_id = "base64:<reducted>"
    secret_access_key = "base64:<reducted>"
    bucket_name = "op-us-west-01-backups"
    folder = "op-us-west-01-01-rke"
  }
}

output_path = "output"
