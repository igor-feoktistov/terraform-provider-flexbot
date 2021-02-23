# cloud-init templates

FlexBot supports `cloud-init` templates in GoLang template format.
The following is a pseudo-structure which is a translation from `flexbot` internal representation to terraform schema via HCL.
All variables of this pseudo-structure can be used in cloud-init templates.
See [ubuntu-18.04.05-cloud-init.template](./ubuntu-18.04.05-cloud-init.template) as an example.

```hcl
Compute = { //HCL - compute
  HostName = "k8s-node1" //HCL - hostname
}

Network = { //HCL - network
  Node = [  //HCL - node
    {
      Name       = "eth2"                  //HCL - name
      Macaddr    = "00:25:b5:99:02:df"     //HCL - macaddr, computed
      Ip         = "192.168.1.25"          //HCL - ip, computed
      Fqdn       = "k8s-node1.example.com" //HCL - fqdn, computed
      Subnet     = "192.168.1.0/24"        //HCL - subnet
      NetLen     = "24"                    //computed
      Gateway    = ""192.168.1.1"          //HCL - gateway
      DnsServer1 = "192.168.1.10"          //HCL - dns_server1
      DnsServer2 = "192.168.4.10"          //HCL - dns_server2
      DnsDomain  = "example.com"           //HCL - dns_domain
    }
  ]
  IscsiInitiator = [ //HCL - iscsi_initiator
    {
      Name          = "iscsi0"                               //HCL - name
      Ip            = "192.168.2.25"                         //HCL - ip, computed
      Fqdn          = "k8s-node1-i1.example.com"             //HCL - fqdn, computed
      Subnet        = "192.168.2.0/24"                       //HCL - subnet
      InitiatorName = "iqn.2005-02.com.open-iscsi:k8s-node1" //computed
      IscsiTarget   = {
        NodeName = "iqn.1992-08.com.netapp:sn.cfe29...<skip>:vs.32" //computed
        Interfaces [
          "iscsi-lif1" //computed
          "iscsi-lif2" //computed
        ]
      }
    },
    {
      Name          = "iscsi1"                               //HCL - name
      Ip            = "192.168.3.25"                         //HCL - ip, computed
      Fqdn          = "k8s-node1-i2.example.com"             //HCL - fqdn, computed
      Subnet        = "192.168.3.0/24"                       //HCL - subnet
      InitiatorName = "iqn.2005-02.com.open-iscsi:k8s-node1" //computed
      IscsiTarget   = {
        NodeName = "iqn.1992-08.com.netapp:sn.cfe29...<skip>:vs.32" //computed
        Interfaces [
          "iscsi-lif3" //computed
          "iscsi-lif4" //computed
        ]
      }
    }
  ]
}

CloudArgs = { //HCL - cloud_args
  //user defined key/value pairs
  cloud_user = "ubuntu"
  ssh_pub_key = "ssh-rsa AAAAN3NyaC2yc3EAAAADR...<skip>"
}
```
