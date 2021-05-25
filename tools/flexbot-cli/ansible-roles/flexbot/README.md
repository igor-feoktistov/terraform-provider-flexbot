flexbot
=======

An Ansible role to provision/deprovision/start/stop/restore bare-metal servers in FlexPod (Cisco UCS and NetAp cDOT)

See flexbot project https://github.com/igor-feoktistov/terraform-provider-flexbot for more details.

Role Variables
--------------
```
op: <operation (provisionServer, deprovisionServer, stopServer, startServer, createSnapshot, deleteSnapshot, restoreSnapshot)>
host: <compute node name>
image: <boot image name>
snapshot: <storage snapshot name>
template: <cloud-init template name (taken from role `templates` if no full path provided)>
flexbotConfig: <flexbot configuration (see `flexbot` CLI tool for more details)>
```

Example Playbooks
-----------------

provisionServer
---------------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Provision server
      vars:
        op: "provisionServer"
        host: "{{ host }}"
        image: "{{ image }}"
        template: "{{ template }}"
        flexbotConfig:
          ipam:
            provider: "Infoblox"
            ibCredentials:
                host: "ib.example.com"
                user: "admin"
                password: "xxxxxx"
                wapiVersion: "2.5"
                dnsView: "Internal"
                networkView: "default"
            dnsZone: "example.com"
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
            spTemplate: "org-root/org-Kubernetes/ls-K8S-01"
            bladeSpec:
                model: "UCSB-B200-(M4|M5)"
                totalMemory: "65536-524289"
          storage:
            cdotCredentials:
                host: "svm.example.com"
                user: "vsadmin"
                password: "xxxxxx"
            bootLun:
                size: 20
            dataLun:
                size: 50
          network:
            node:
              - name: "eth2"
                subnet: "192.168.1.0/24"
                gateway: "192.168.1.1"
                dnsServer1: "192.168.1.10"
                dnsDomain: "example.com"
            iscsiInitiator:
              - name: "iscsi0"
                subnet: "192.168.2.0/24"
              - name: "iscsi1"
                subnet: "192.168.2.0/24"
          cloudArgs:
            ssh_pub_key: ""ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
      import_role:
        name: flexbot
    - name: Display server IP address from server_response
      debug:
        msg: "eth2.ip={{ server_response.network.node[0].ip }}"
```

deprovisionServer
-----------------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Deprovision server
      vars:
        op: "deprovisionServer"
        host: "{{ host }}"
        flexbotConfig:
          ipam:
            provider: "Infoblox"
            ibCredentials:
                host: "ib.example.com"
                user: "admin"
                password: "xxxxxx"
                wapiVersion: "2.5"
                dnsView: "Internal"
                networkView: "default"
            dnsZone: "example.com"
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
          storage:
            cdotCredentials:
                host: "svm.example.com"
                user: "vsadmin"
                password: "xxxxxx"
          network:
            node:
              - name: "eth2"
                subnet: "192.168.1.0/24"
            iscsiInitiator:
              - name: "iscsi0"
                subnet: "192.168.2.0/24"
              - name: "iscsi1"
                subnet: "192.168.3.0/24"
      import_role:
        name: flexbot
    - name: Display server name from server_response
      debug:
        msg: "hostname={{ server_response.compute.hostName }}"
```

stopServer
----------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Stop server
      vars:
        op: "stopServer"
        host: "{{ host }}"
        flexbotConfig:
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
      import_role:
        name: flexbot
    - name: Display server name from server_response
      debug:
        msg: "hostname={{ server_response.compute.hostName }}"
```

startServer
-----------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Start server
      vars:
        op: "startServer"
        host: "{{ host }}"
        flexbotConfig:
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
      import_role:
        name: flexbot
    - name: Display server name from server_response
      debug:
        msg: "hostname={{ server_response.compute.hostName }}"
```

createSnapshot
--------------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Create snapshot
      vars:
        op: "createSnapshot"
        host: "{{ host }}"
        snapshot: "{{ snapshot_name }}"
        flexbotConfig:
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
          storage:
            cdotCredentials:
                host: "svm.example.com"
                user: "vsadmin"
                password: "xxxxxx"
      import_role:
        name: flexbot
```

deleteSnapshot
--------------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Delete snapshot
      vars:
        op: "deleteSnapshot"
        host: "{{ host }}"
        snapshot: "{{ snapshot_name }}"
        flexbotConfig:
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
          storage:
            cdotCredentials:
                host: "svm.example.com"
                user: "vsadmin"
                password: "xxxxxx"
      import_role:
        name: flexbot
```

restoreSnapshot
---------------
```
- hosts: localhost
  gather_facts: no
  connection: local
  tasks:
    - name: Restore snapshot
      vars:
        op: "restoreSnapshot"
        host: "{{ host }}"
        snapshot: "{{ snapshot_name }}"
        flexbotConfig:
          compute:
            ucsmCredentials:
                host: "ucsm.example.com"
                user: "admin"
                password: "xxxxxx"
            spOrg: "org-root/org-Kubernetes"
          storage:
            cdotCredentials:
                host: "svm.example.com"
                user: "vsadmin"
                password: "xxxxxx"
      import_role:
        name: flexbot
```
