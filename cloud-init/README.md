# cloud-init templates

FlexBot supports `cloud-init` templates in GoLang template format.
The following is a pseudo-structure which is a translation from `terraform` schema via HCL to `flexbot` internal representation.
This pseudo-structure is presented in HCL just to abstract you from GoLang and focus on Terraform only.
All variables of this pseudo-structure can be used in cloud-init templates.
See [ubuntu-20.04.04-cloud-init.template](./ubuntu-20.04.04-cloud-init.template) as an example cloud-init template for Rancher RKE node.

```hcl
Compute = { //schema - compute
  HostName = "k8s-node1" //schema - hostname
}

Network = { //schema - network
  Node = [  //schema - node (list)
    {
      Name       = "eth2"                  //schema - name
      Macaddr    = "00:25:b5:99:02:df"     //schema - macaddr, computed
      Ip         = "192.168.1.25"          //schema - ip, computed
      Fqdn       = "k8s-node1.example.com" //schema - fqdn, computed
      Subnet     = "192.168.1.0/24"        //schema - subnet
      NetLen     = "24"                    //computed
      Gateway    = "192.168.1.1"           //schema - gateway
      DnsServer1 = "192.168.1.10"          //schema - dns_server1
      DnsServer2 = "192.168.4.10"          //schema - dns_server2
      DnsServer3 = "192.168.5.10"          //schema - dns_server3
      DnsDomain  = "example.com"           //schema - dns_domain
      Parameters = {                       //schema - parameters
        mtu = "9000"
      }
    }
  ]
  IscsiInitiator = [ //schema - iscsi_initiator (list)
    {
      Name          = "iscsi0"                                      //schema - name
      Macaddr       = "00:25:b5:99:00:7f"                           //schema - macaddr, computed
      Ip            = "192.168.2.25"                                //schema - ip, computed
      Fqdn          = "k8s-node1-i1.example.com"                    //schema - fqdn, computed
      Subnet        = "192.168.2.0/24"                              //schema - subnet
      InitiatorName = "iqn.2005-02.com.open-iscsi:k8s-node1"        //schema - initiator_name, computed
      IscsiTarget   = {                                             //schema - iscsi_target, computed
        NodeName = "iqn.1992-08.com.netapp:sn.cfe29...<skip>:vs.32" //schema - node_name, computed
        Interfaces = [                                              //schema - interfaces, computed
          "iscsi-lif1"
          "iscsi-lif2"
        ]
      }
    }
    {
      Name          = "iscsi1"                                      //schema - name
      Macaddr       = "00:25:b5:99:09:5f"                           //schema - macaddr, computed
      Ip            = "192.168.3.25"                                //schema - ip, computed
      Fqdn          = "k8s-node1-i2.example.com"                    //schema - fqdn, computed
      Subnet        = "192.168.3.0/24"                              //schema - subnet
      InitiatorName = "iqn.2005-02.com.open-iscsi:k8s-node1"        //schema - initiator_name, computed
      IscsiTarget   = {                                             //schema - iscsi_target, computed
        NodeName = "iqn.1992-08.com.netapp:sn.cfe29...<skip>:vs.32" //schema - node_name, computed
        Interfaces = [                                              //schema - interfaces, computed
          "iscsi-lif3"                                              //schema - interface ip, computed
          "iscsi-lif4"                                              //schema - interface ip, computed
        ]
      }
    }
  ]
  NvmeHost = [ //schema - nvme_host (list)
    {
      HostInterface = "iscsi0"                                       //schema - host_interface
      Ip            = "192.168.2.25"                                 //schema - ip, computed
      Subnet        = "192.168.2.0/24"                               //schema - subnet, computed
      HostNqn       = "nqn.2014-08.org.nvmexpress:uuid:45c8<skip>"   //schema - host_nqn, computed
      NvmeTarget    = {                                              //schema - nvme_target, computed
        TargetNqn = "nqn.1992-08.com.netapp:sn.<skip>"               //schema - target_nqn, computed
        Interfaces = [                                               //schema - interfaces, computed
          "nvme-lif1"                                                //schema - interface ip, computed
          "nvme-lif2"                                                //schema - interface ip, computed
        ]
      }
    }
    {
      HostInterface = "iscsi1"                                       //schema - host_interface
      Ip            = "192.168.3.25"                                 //schema - ip, computed
      Subnet        = "192.168.3.0/24"                               //schema - subnet, computed
      HostNqn       = "nqn.2014-08.org.nvmexpress:uuid:45c8<skip>"   //schema - host_nqn, computed
      NvmeTarget    = {                                              //schema - nvme_target, computed
        TargetNqn = "nqn.1992-08.com.netapp:sn.<skip>"               //schema - target_nqn, computed
        Interfaces = [                                               //schema - interfaces, computed
          "nvme-lif1"                                                //schema - interface ip, computed
          "nvme-lif2"                                                //schema - interface ip, computed
        ]
      }
    }
  ]
}

Storage = { //schema - storage
  BootLun = { //schema - boot_lun
    Name = "k8s_node1_boot" //schema - name, computed
    Id = 0                  //schema - id, computed
    Size = 32               //schema - size
  }
  DataLun = { //schema - data_lun
    Name = "k8s_node1_data" //schema - name, computed
    Id = 1                  //schema - id, computed
    Size = 128              //schema - size
  }
  DataNvme = { //schema - data_nvme
    Namespace = "k8s_node1_data" //schema - namespace name, computed
    Subsystem = "k8s_node1_data" //schema - subsystem name, computed
    Size = 128                   //schema - size
  }
}

CloudArgs = { //schema - cloud_args
  //user defined key/value pairs
  cloud_user = "ubuntu"
  ssh_pub_key = "ssh-rsa AAAAN3NyaC2yc3EAAAADR...<skip>"
}
```
