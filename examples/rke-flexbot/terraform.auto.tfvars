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
      },
      {
        name = "iscsi1"
        subnet = "192.168.4.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_server3 = ""
        dns_domain = ""
      }
    ]
  }
  storage = {
    api_method = "zapi"
    zapi_version = ""
  }
}

rke_config = {
  cluster_name = "onprem-us-east-1-01"
  rke_version = "v1.20.11-rancher1-1"
  docker_version = "20.10"
  rancher_server = "onprem-us-east-1.rancher.example.com"
  api_url = "https://192.168.2.22:6443"
  server_ca_data = "LS0wLS3CRUdJT<...reducted...>"
  client_cert_data = "LS0wLS5CHUd<...reducted...>"
  client_key_data = "base64:DoPqKyrW<...reducted...>"
  s3_backup = {
    region = "us-east-1"
    endpoint = "s3-accesspoint.us-east-1.amazonaws.com"
    access_key_id = "base64:DoPerW<...reducted...>"
    secret_access_key = "base64:Z9sdQw<...reducted...>"
    bucket_name = "onprem-us-east-1-backups"
    folder = "onprem-us-east-1-01-rke"
  }
}

output_path = "output"
