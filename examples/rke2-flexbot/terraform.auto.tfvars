flexbot_credentials = {
  infoblox = {
    host = "192.168.10.20"
    user = "base64:z12Wq4rOp<..reducted...>"
    password = "base64:8KJJadwiFWL6iyY0<..reducted...>"
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "base64:a9cNWsDDxsrib<..reducted...>"
    password = "base64:9KLx1FGorXeFgKU<..reducted...>"
  }
  cdot = {
    host = "svm.example.com"
    user = "base64:LsaMU7KJiR<..reducted...>"
    password = "base64:7WVf+jcVrH2T<..reducted...>"
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
    ssh_user = "cloud-user"
    ssh_public_key = "base64:OjdfgfrtyGBWGHZlI<..reducted...>"
    ssh_private_key = "base64:4aA5nbdBCwZq<..reducted...>"
  }
  network = {
    node = [
      {
        name = "eth2"
        subnet = "192.168.2.0/24"
        gateway = "192.168.2.1"
        dns_server1 = "192.168.2.10"
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = "example.com"
        parameters = {}
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
        parameters = {
          mtu = "9000"
        }
      },
      {
        name = "iscsi1"
        subnet = "192.168.4.0/24"
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
  }
  storage = {
    api_method = "rest"
  }
}

rke2_config = {
  rke2_cluster = "onprem-us-east-1-01"
  rke2_version = "v1.31.1+rke2r1"
  rke2_token = "<reducted>"
  rke2_server = "192.168.2.20"
  server_ca_data = "LS0wLS3CRUdJT<...reducted...>"
  client_cert_data = "LS0wLS5CHUd<...reducted...>"
  client_key_data = "base64:DoPqKyrW<...reducted...>"
  s3_backup = {
    region = "us-east-1"
    endpoint = "s3-accesspoint.us-east-1.amazonaws.com"
    access_key_id = "base64:<...reducted...>"
    secret_access_key = "base64:<...reducted...>"
    bucket = "onprem-us-east-1-backups"
    folder = "onprem-us-east-1-01-rke2"
  }
}

output_path = "output"
