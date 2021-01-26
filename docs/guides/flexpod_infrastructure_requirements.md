# Infrastructure Requirements

## cDOT Storage Requirements

#### iSCSI LIF's

Minimal one iSCSI LIF is required. Two or four LIF's in two VLAN ID's are highly recommended.
Non-routed iSCSI is highly recommeneded, meaning interfaces for host iSCSI initiator and SVM
iSCSI LIF's should belong to the same VLAN's.

#### NAS LIF's

At least one NFS LIF is required for image repo management functionality.
Once all images are uploaded, NAS LIF can be disabled or removed.
Server functionality does not require this LIF.

## UCS Requirements

UCS Service Profile Template should be configured to support iSCSI boot.

See below screenshots with configuration details.

### iSCSI vNIC's

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details1.png">
</p>

### Boot Order

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details2.png">
</p>

### iSCSI Boot Parameters

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details3.png">
</p>

## Images

There is no requirements on which tool to use to create images.
Though I highly recommend Diskimage-builder from OpenStack project `https://docs.openstack.org/diskimage-builder/latest/`.
See `examples/diskimage-builder/ubuntu-18.04-iboot.sh` on how to build iSCSI booted ubuntu-18.04.
To manage images and cloud-init templates, use repo resource. Make sure to run provider in local to your FlexPod network.
The provider is using NFS protocol to upload images. There is no requirements for NFS client.
NFS client support in the provider is built-in.

## Cloud-init templates

Cloud-init templates are GoLang templates with passed from `configuration` parameters.
See `examples/cloud-init/ubuntu-18.04-cloud-init.template` as an example for ubuntu-18.04.
Cloud-init templates can be kept in storage repository similarly to images.
