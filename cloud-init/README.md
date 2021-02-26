# cloud-init templates

FlexBot supports `cloud-init` templates in GoLang template format.
The following is a pseudo-structure which is a translation from `terraform` schema via HCL to `flexbot` internal representation.
This pseudo-structure is presented in HCL just to abstract you from GoLang and focus on Terraform only.
All variables of this pseudo-structure can be used in cloud-init templates.
See [ubuntu-18.04.05-cloud-init.template](./ubuntu-18.04.05-cloud-init.template) as an example cloud-init template for Rancher RKE node.

```hcl
Compute = { //schema - compute
  HostName = "k8s-node1" //schema - hostname
}

Network = { //schema - network
  Node = [  //schema - node
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
      DnsDomain  = "example.com"           //schema - dns_domain
    }
  ]
  IscsiInitiator = [ //schema - iscsi_initiator
    {
      Name          = "iscsi0"                                      //schema - name
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
      Ip            = "192.168.3.25"                                //schema - ip, computed
      Fqdn          = "k8s-node1-i2.example.com"                    //schema - fqdn, computed
      Subnet        = "192.168.3.0/24"                              //schema - subnet
      InitiatorName = "iqn.2005-02.com.open-iscsi:k8s-node1"        //schema - initiator_name, computed
      IscsiTarget   = {                                             //schema - iscsi_target, computed
        NodeName = "iqn.1992-08.com.netapp:sn.cfe29...<skip>:vs.32" //schema - node_name, computed
        Interfaces = [                                              //schema - interfaces, computed
          "iscsi-lif3"
          "iscsi-lif4"
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
}

CloudArgs = { //schema - cloud_args
  //user defined key/value pairs
  cloud_user = "ubuntu"
  ssh_pub_key = "ssh-rsa AAAAN3NyaC2yc3EAAAADR...<skip>"
}
```
