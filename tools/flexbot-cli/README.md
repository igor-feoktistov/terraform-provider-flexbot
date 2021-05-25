# flexbot

`flexbot` is a CLI alternative to `terraform-provider-flexbot` to build and manage bare-metal Linux on [FlexPod](https://flexpod.com) (Cisco UCS and NetApp cDOT).
It can be used in other tools like ansible (see [flexbot](./ansible-roles/flexbot) ansible role for more details).

## Building `flexbot` CLI tool

* Clone [terraform-provider-flexbot project repository](https://github.com/igor-feoktistov/terraform-provider-flexbot) to: `$GOPATH/src`.
* Enter `$GOPATH/src/terraform-provider-flexbot/tools/flexbot-cli` directory.
* Run `make` to build CLI binary for your platform.
* Run `make ansible` to build CLI binaries for `flexbot` ansible role.

## Usage

 - Provision server:\
   ```flexbot --config=<config file path> --op=provisionServer --host=<host name> --image=<image name> --template=<cloud-init template name or path>```

 - De-provision server:\
   ```flexbot --config=<config file path> --op=deprovisionServer --host=<host name>```

 - Power Off server:\
   ```flexbot --config=<config file path> --op=stopServer --host=<host name>```

 - Power On server:\
   ```flexbot --config=<config file path> --op=startServer --host=<host name>```

 - Create cDOT snapshot:\
   ```flexbot --config=<config file path> --op=createSnapshot --host=<host name> --snapshot=<snapshost name>```

 - Delete cDOT snapshot:\
   ```flexbot --config=<config file path> --op=deleteSnapshot --host=<host name> --snapshot=<snapshost name>```

 - Restore host from cDOT snapshot:\
   ```flexbot --config=<config file path> --op=restoreSnapshot --host=<host name> --snapshot=<snapshost name>```

 - List of available storage snapshots:\
   ```flexbot --config=<config file path> --op=listSnapshots --host=<host name>```

 - Upload image into image repository:\
   ```flexbot --config=<config file path> --op=uploadImage --image=<image name> --imagePath=<image path>```

 - Delete image from image repository:\
   ```flexbot --config=<config file path> --op=deleteImage --image=<image name>```

 - List images in image repository:\
   ```flexbot --config=<config file path> --op=listImages```

 - Upload cloud-init template into template repository:\
   ```flexbot --config=<config file path> --op=uploadTemplate --template=<template name> --templatePath=<template path>```

 - Download cloud-init template from template repository and print to STDOUT:\
   ```flexbot --config=<config file path> --op=downloadTemplate --template=<template name>```

 - Delete cloud-init template from template repository:\
   ```flexbot --config=<config file path> --op=deleteTemplate --template=<template name>```

 - List cloud-init templates in template repository:\
   ```flexbot --config=<config file path> --op=listTemplates```

 - Encrypt passwords in configuration:\
   ```flexbot --config=<config file path> --op=encryptConfig [--passphrase=<password phrase>]```

 - Decrypt passwords in configuration:\
   ```flexbot --config=<config file path> --op=decryptConfig [--passphrase=<password phrase>]```

 - Encrypt string:\
   ```flexbot --op=encryptString --sourceString <string to encrypt> [--passphrase=<password phrase>]```

## Runtime arguments

  - config: `a path to configuration file, STDIN, or argument value in JSON (default is "STDIN")`
  - dumpResult: `file path or STDOUT (default is "STDOUT")`
  - encodingFormat: `supported encoding formats: json, yaml (default "yaml")`
  - host: `compute node name`
  - image: `boot image name`
  - imagePath: `a path to boot image (optional prefix can be either file:// or http(s)://)`
  - template: `cloud-init template name or path (optional prefix can be either file:// or http(s)://)`
  - templatePath: `cloud-init template path (optional prefix can be either file:// or http(s)://)`
  - snapshot: `storage snapshot name - in cDOT storage it is a volume snapshot name`
  - op: `provisionServer, deprovisionServer, stopServer, startServer, createSnapshot, deleteSnapshot, restoreSnapshot, listSnapshots, uploadImage, deleteImage, listImages, uploadTemplate, downloadTemplate, deleteTemplate, listTemplates, encryptConfig, decryptConfig, encryptString`
  - sourceString: `source string to encrypt by encryptString operation`
  - passphrase: `passphrase to encrypt/decrypt passwords in configuration (default is machine ID)`

## Passwords Encryption

Your host `machineid` is a default encryption key if you choose to encrypt passwords.

You may also want to use `encryptString` operation to generate encrypted passwords values.

## Configuration

Configuration can be provided either in YAML or JSON format.

```
# IPAM is implemented via pluggable providers.
# Only Infoblox and Internal providers are supported at this time.
# Internal provider expects you to supply "ip" and "fqdn" in network configuration.
ipam:
    provider: Infoblox
    # Credentials for Infoblox master
    ibCredentials:
        host: ib.example.com
        user: admin
        # if you choose to encrypt passwords, should start from "base64:" prefix
        password: secret
        wapiVersion: "2.5"
        dnsView: Internal
        networkView: default
    # Compute node FQDN is <node name>.<dnsZone>
    dnsZone: example.com
# UCS Service Profile is created from Service Profile Template (SPT)
compute:
    # Credentials for UCSM
    ucsmCredentials:
        host: ucsm.example.com
        user: admin
        password: secret
    # UCS Service Profile (server) is to be created here
    spOrg: org-root/org-Kubernetes
    # Reference to Service Profile Template (SPT)
    spTemplate: org-root/org-Kubernetes/ls-K8S-SubProd-01
    # Blade search is conducted by applying "AND" rule to all provided specs
    bladeSpec:
        # "dn" is optional, supports regexp
        #dn: sys/chassis-1/blade-2
        #dn: sys/chassis-9/blade-[0-9]+
        # "model" is optional, supports regexp
        model: UCSB-B200-M4
        #model: UCSB-B200-M[45]
        # "numOfCpus" is optional, supports ranges
        numOfCpus: "2"
        # "numOfCores" is optional, support ranges
        numOfCores: "36"
        # "totalMemory" in MB is optional, supports ranges
        totalMemory: "262144"
        #totalMemory: "262144-393216"
storage:
    # Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended
    cdotCredentials:
        host: svm.example.com
        user: vsadmin
        password: secret
        # ZAPI version to handle older OnTap (optional, default is "1.160")
        zapiVersion: "1.110"
        # API method ("zapi" or "rest", default is "zapi")
        apiMethod: "zapi"
    # not required if SVM is in cdotCredentials
    #svmName: svmlabk8s03spd
    # Boot LUN
    bootLun:
        # boot LUN size in GB
        size: 20
    # Data LUN (optional)
    dataLun:
        # data LUN size in GB
        size: 50
    # Seed LUN (optional)
    seedLun:
        # optionally you can pass seedTemplate location here
        seedTemplate:
          # see "template" runtime argument
          location: templates/ubuntu-18.04-cloud-init.template
network:
    # Node network interfaces (list)
    node:
        # name should match respective vNIC name in SPT
      - name: eth2
        # Supply IP here either for Internal provider or for static IP assignment in Infoblox
        ip: 192.168.1.52
        # Supply FQDN here only for Internal provider
        fqdn: k8s-node1.example.com
        # IPAM allocates IP for node interface
        subnet: 192.168.1.0/24
        # ipRange (optional) will be used by IPAM instead of subnet to allocate IP from.
        # ipRange start/end IP's should match respective start/end IP's in Infoblox IP range
        #ipRange: 192.168.1.1-192.168.1.64
        gateway: 192.168.1.1
        # arguments for node resolver configuration
        dnsServer1: 192.168.1.10
        # "dnsServer2" is optional
        dnsServer2: 192.168.2.10
        # "dnsServer3" is optional
        dnsServer3: 192.168.3.10
        dnsDomain: example.com
    # iSCSI initiator network interfaces (list)
    # Minimum one interface required, two is highly recommended
    iscsiInitiator:
        # name should match respective iSCSI vNIC name in SPT
      - name: iscsi0
        # Supply IP here either for Internal provider or for static IP assignment in Infoblox
        ip: 192.168.2.80
        # Supply FQDN here only for Internal provider
        fqdn: k8s-node1-i1.example.com
        # IPAM allocates IP for iSCSI interface
        subnet: 192.168.2.0/24
        # name should match respective iSCSI vNIC name in SPT
      - name: iscsi1
        # Supply IP here either for Internal provider or for static IP assignment in Infoblox
        ip: 192.168.3.78
        # Supply FQDN here only for Internal provider
        fqdn: k8s-node1-i2.example.com
        # IPAM allocates IP for iSCSI interface
        subnet: 192.168.3.0/24
cloudArgs:
    # optional user defined key/value pairs to address in cloud-init templates
    cloud_user: cloud-user
    ssh_pub_key: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC9W8<trimmed>"
```

## Command Output

Command output (either in YAML or JSON format) will include submitted configuration plus discovered
while build host and storage entities. This is an example output from successful run:
```
# status is either "success" or "failure"
# "errorMessage" will be added in case of "failure"
status: success
server:
    ipam:
        provider: Infoblox
        dnsZone: example.com
    compute:
        hostName: k8s-node1
        spOrg: org-root/org-Kubernetes
        spTemplate: org-root/org-Kubernetes/ls-K8S-SubProd-01
        spDn: org-root/org-Kubernetes/ls-k8s-node1
        bladeSpec:
            dn: sys/chassis-1/blade-5
            model: UCSB-B200-M4
            numOfCpus: 2
            numOfCores: 36
            totalMemory: 65536
    storage:
        svmName: svmlabk8s03spd
        imageRepoName: image_repo
        volumeName: k8s_node1_iboot
        igroupName: k8s_node1_iboot
        bootLun:
            name: k8s_node1_iboot
            size: 20
            osImage:
                name: ubuntu-18.04-iboot
        dataLun:
            name: k8s_node1_data
            id: 1
            size: 50
        seedLun:
            name: k8s_node1_seed
            id: 2
            seedTemplate:
                location: templates/ubuntu-18.04-cloud-init.template
    network:
        node:
          - name: eth2
            macaddr: 00:25:B5:99:04:BF
            ip: 192.168.1.52
            fqdn: k8s-node1.example.com
            subnet: 192.168.1.0/24
            netlen: "24"
            gateway: 192.168.1.1
            dnsServer1: 192.168.1.10
            dnsDomain: example.com
        iscsiInitiator:
          - name: iscsi0
            ip: 192.168.2.80
            fqdn: k8s-node1-i1.example.com
            subnet: 192.168.2.0/24
            netlen: "24"
            gateway: 0.0.0.0
            dnsServer1: 0.0.0.0
            dnsServer2: 0.0.0.0
            initiatorName: iqn.2005-02.com.open-iscsi:k8s-node1.1
            iscsiTarget:
                nodeName: iqn.1992-08.com.netapp:sn.cfe29c87000211eabab300a098ae4dc7:vs.32
                interfaces:
                  - 192.168.2.58
                  - 192.168.2.57
          - name: iscsi1
            ip: 192.168.3.78
            fqdn: k8s-node1-i2.example.com
            subnet: 192.168.3.0/24
            netlen: "24"
            gateway: 0.0.0.0
            dnsServer1: 0.0.0.0
            dnsServer2: 0.0.0.0
            initiatorName: iqn.2005-02.com.open-iscsi:k8s-node1.2
            iscsiTarget:
                nodeName: iqn.1992-08.com.netapp:sn.cfe29c87000211eabab300a098ae4dc7:vs.32
                interfaces:
                  - 192.168.3.58
                  - 192.168.3.57
```
