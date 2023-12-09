## 1.9.5 (December 8, 2023)

ENHANCEMENTS:
* Enhancements to support Rancher Management Server 2.7.* and downstream RKE2 clusters
  * Rancher 2.7.* replaces Node resource with Machine resource in node management for RKE2 clusters
  * Node removal via Norman API will not remove Machine resource, therefore, requires special handling

## 1.9.4 (August 31, 2023)

BUG FIXES:
* Fix for the issue when node taints may not be properly assigned in certain conditions


## 1.9.3 (August 29, 2023)

ENHANCEMENTS:
* Further improvements in NVME over TCP support for data disk.
  * Simplified switching from iSCSI to NVME for data disk
  * Updated example for RKE2 downstream cluster with data disk on NVME
  * Updated elements of diskimage-builder for ubuntu-22.04

BUG FIXES:
* Fix for the issue with RKE2 nodes upgrades when labels and taints were not consistently applied.


## 1.9.2 (April 4, 2023)

BUG FIXES:
* Fix for the issue with REST LunCopy routine for the case of substantially delayed response from a storage.


## 1.9.1 (April 3, 2023)

ENHANCEMENTS:
* Standardize and increase wait timeouts in waiting routines for cluster or nodes states.
  * Reduce impact on Rancher management server in case of long list of nodes waiting for state change.


## 1.9.0 (March 29, 2023)

ENHANCEMENTS:
* NVME over TCP support for data disk.
  * Requires `api_method=rest` and OnTap v9.12.1 or higher.
  * Requires ubuntu-22.04.2 or higher.
  * See resource documentation for more details.
* Cluster scope for `api_method=rest`
  * Requires resource setting `storage`->`svm_name`
  * See resource documentation for more details.


## 1.8.4 (February 14, 2023)

ENHANCEMENTS:
* Minor enhancements in `rancher_api` code to support nodes upgrades for RKE2
* Added diskimage-builder elements and build script for ubuntu-22.04


## 1.8.3 (December 16, 2022)

BUG FIXES:
* Minor issue in `ranche2-client.NodeCordonDrain` routine for scenario when drain timeout exceeded.


## 1.8.2 (December 14, 2022)

ENHANCEMENTS:
* Multiple improvements in ONTAP REST API client
  * api_method=`rest` is stable now, requires OnTap v9.12 or higher

## 1.8.1 (October 18, 2022)

BUG FIXES:
* Minor issue in `Read` routine for scenario when rancher_api is enabled and node failed to join the cluster


## 1.8.0 (October 12, 2022)

ENHANCEMENTS:
* GoLang v1.19
* Support for standard Kubernetes API (RKE) in ***rancher_api*** provider configuration:
  * Node operations `cordon`, `uncordon`, `drain`.
  * Maintain nodes `labels` and `taints`.
  * See resource documentation and example HCL's for more details.
* Drift detection for node labels and taints if ***rancher_api*** is configured
  * ***apply*** action would remediate the drift
* Transient error resiliency in ***rancher_api***
  * make sure to set `retries` parameter accordingly
  * See resource documentation for more details.
* Updates in example HCL's
* Fixes in diskimage-builder elements for ubuntu-20.04


## 1.7.19 (September 6, 2022)

ENHANCEMENTS:
* Increase default timeouts for Create and Update routines.


## 1.7.18 (June 7, 2022)

FEATURES:
* **New Resource Argument:** **taints** - (Optional) allows to set and manage k8s node taints, requires Rancher API enabled.
  * See resource documentation for more details.


## 1.7.17 (May 11, 2022)

ENHANCEMENTS:
* **New argument:** `storage`/`force_update`:
  * Optional argument to force node re-imaging.
  * See resource documentation for more details.


## 1.7.16 (May 9, 2022)

ENHANCEMENTS:
* Encryption support for `pass_phrase` provider argument:
  * Encryption key can be one of the following:
    * machineID() if `pass_phrase_env_key` is not provided
    * ENV variable `pass_phrase_env_key` value
  * See [flexbot-crypt](./tools/flexbot-crypt) utility
* **New provider argument:** `pass_phrase_env_key`:
  * Optional argument to provide ENV variable name for passing encryption key
  * See provider documentation for more details.


## 1.7.15 (March 2, 2022)

ENHANCEMENTS:
* SSH timeout for dial method to prevent ssh hangs under certain conditions.


## 1.7.14 (February 9, 2022)

ENHANCEMENTS:
* **New Argument:** `maintenance` - (Optional) executes maintenance tasks on the node.
  See resource documentation for more details.


## 1.7.12 (January 31, 2022)

ENHANCEMENTS:
* Check if physical blade per new bladeSpec is available prior to node cordon/drain calls
* Re-check node powerstate right before applying new bladeSpec


## 1.7.11 (December 15, 2021)

ENHANCEMENTS:
* Workaround for concurency issue (concurrent map read and map write) in terraform upstream code
* Adjustments in OnTap API code to mitigate "context deadline exceeded" in ZAPI calls


## 1.7.10 (December 10, 2021)

ENHANCEMENTS:
* Adjustments in Rancher API code to fix node update issues in Rancher 2.6.*:
  * node shutdown is delayed until cluster completes updates to allow catlle-node-cleanup job be executed


## 1.7.9 (November 30, 2021)

ENHANCEMENTS:
* Improvements in rancher API code:
  * wait for node registration to finish until node is in "active" state
  * make sure that during node_grace_timeout wait node is in "active" state
* Improvements in OnTap API code:
  * no error response for NotFound errors in delete's to deal with occasional storage leftovers


## 1.7.8 (October 29, 2021)

ENHANCEMENTS:
* Updated go.mod to support latest Rancher client.
* The source code is formatted by "gofmt".
* The source code is "staticcheck" compliant.

FEATURES:
* Built-in "decrypt" support for "ssh_private_key" attribute:
  * encrypted "ssh_private_key" attribute will stay encrypted in tfstate file
  * see [flexbot-crypt](./tools/flexbot-crypt) utility
* Built-in "decrypt" support for values in "cloud_args" attribute:
  * any values in "cloud_args" can be encryped and therefore stay encrypted in tfstate file
  * see [flexbot-crypt](./tools/flexbot-crypt) utility


## 1.7.7 (July 29, 2021)

ENHANCEMENTS:
* Updated go.mod to support Rancher latest client.

BUG FIXES:
* Set default value for network node interface parameters to {} which fixes warnings "Objects have changed outside of Terraform".
* Fix for the issue in `resourceUpdateServer` routine when updated BladeSpec combined with requested powerstate="down" would cause failure in rancher.NodeWaitForState().


## 1.7.6 (July 2, 2021)

BUG FIXES:
* Fix for the issue in `createSnapshot` routine when in certain filesystem layouts `fsfreeze` may fail.


## 1.7.5 (June 25, 2021)

FEATURES:
* New Resource Arguments:
  * compute.label - (Optional) allows to set and manage UCS Service Profile label
  * compute.description - (Optional) allows to set and manage UCS Service Profile description


## 1.7.4 (June 10, 2021)

ENHANCEMENTS:
* Migrated provider to Terraform Plugin SDK v2

FEATURES:
* New Resource Argument: **labels** - (Optional) allows to set and manage k8s node labels, requires Rancher API enabled.


## 1.7.3 (May 25, 2021)

BUG FIXES:
* Fix for provider panic condition while server refresh when UCS service profile does not have physical blade assigned

ENHANCEMENTS:
* CLI tool `flexbot` is an alternative to `terraform-provider-flexbot` to build and manage bare-metal Linux on FlexPod.
  It can be used in other tools like ansible (see ansible role for `flexbot`).


## 1.7.2 (May 11, 2021)

BUG FIXES:
* Fix in `server` schema to allow empty string in network.node[*].ip


## 1.7.1 (April 20, 2021)

ENHANCEMENTS:
* IPAM - static IP address can be specified for an interface with Infoblox plugin.
  This will cause specified IP assigned to a host record (if does not exist) rather than allocated from a subnet or IP range.


## 1.7.0 (April 12, 2021)

ENHANCEMENTS:
* This release initiates a transition from ONTAP ZAPI to ONTAP REST API.
  ZAPI is still default and stable method. REST API is experimental for now.
* Storage efficiency settings in ONTAP volume and LUN creation calls.

FEATURES:
* **New Provider Argument:** `storage.credentials.api_method` - (Optional) Allowed values "zapi" and "rest". Default value is "zapi".


## 1.6.8 (April 6, 2021)

ENHANCEMENTS:
* New server attribute `network.node.parameters` is a map with user defined key/value pairs to resolve in cloud-init template network interface settings.
  See examples for more details.
* New server attribute `network.node.dns_server3 to define third DNS server in node resolver configuration.`


## 1.6.7 (March 29, 2021)

ENHANCEMENTS:
* New server attribute (computed) `blade_spec.blade_assigned.serial` captures blade serial number.

BUG FIXES:
* Fix in `repo` resource - removed failure condition for a brand new repo when repo volume does not exist yet.


## 1.6.6 (February 3, 2021)

FEATURES:
* **New Parameter:** `ip_range` in `compute/network` - (Optional) Allows to allocate IP's from IP range if defined.

ENHANCEMENTS:
* Infoblox IPAM plugin is enhanced with allocation from IP range functionality.
  IP will be allocated from IP range not entire subnet if `ip_range` parameter is defined.
  IP range should belong to specified subnet.

BUG FIXES:


## 1.6.4 (February 1, 2021)

FEATURES:

ENHANCEMENTS:

BUG FIXES:
* Fix in the node cordon/drain routine which caused failures because of drain timeout error from rancher was propagated to node update routine
* Minor fixes in examples


## 1.6.3 (January 26, 2021)

FEATURES:
* This release consolidates source code from `flexbot` project to a single tree on `terraform-provider-flexbot` project and obsoletes `flexbot` CLI tool.

ENHANCEMENTS:

BUG FIXES:


## 1.6.2 (January 14, 2021)

FEATURES:
* **New Parameter:** `wait_for_node_timeout` in `rancher_api` provider argument - (Optional) Ensures Rancher node is "active" before completing.
* **New Parameter:** `num_of_threads` in `blade_spec` compute argument - (Optional) Blade search by range of `num_of_threads` or exact value.
* **New Datasource:** `flexbot_crypt` to help with encrypting user names, passwords, and tokens.

ENHANCEMENTS:
* Record assigned compute details in node annotations (suffix flexpod-compute).
* Record assigned storage details in node annotations (suffix flexpod-storage).
* The above enhancements require `rancher_api` and `wait_for_node_timeout` > 0 in provider settings.
* Package `crypt` is updated to replace md5 with sha256 sum. Make sure to re-generate encrypted strings in credentials.

BUG FIXES:


## 1.6.1 (January 4, 2021)

FEATURES:

ENHANCEMENTS:
* IPAM code optimization to make it more "plugin" friendly.
* Code sync-up with latest changes in kdomanski/iso9660 package.

BUG FIXES:
* Fixed the issue when under certain conditions change in blade_spec would
  trigger blade re-assignment in spite of current blade meets all criteria
  in newly provided blade_spec.


## 1.6.0 (December 18, 2020)

FEATURES:
* **New Resource:** `repo` - Flexbot images and templates repositories management via Terraform

ENHANCEMENTS:
* Added support for Linux on ARM64 platform

BUG FIXES:

## 1.5.4 (December 15, 2020)

FEATURES:
* **New Argument:** `compute/powerstate` - (Optional) Enables powerstate management for UCS blade.

ENHANCEMENTS:

BUG FIXES:

## 1.5.3 (December 8, 2020)

FEATURES:

ENHANCEMENTS:

* Improved Rancher node use case logic for server updates.
* Code optimization for Rancher

BUG FIXES:

## 1.5.1 (December 2, 2020)

FEATURES:
* **New Argument:** `auto_snapshot_on_update` - (Optional) Enables automatic snapshots on image or seed template updates.
* **New Argument:** `restore` - (Optional) Enables restore functionality for server LUN's.
* **New Argument:** `rancher_api/enabled` - (Optional) Gives a flexibility to define `rancher_api` which is not functional yet (spin-up Rancher Management Server as an example).

ENHANCEMENTS:
* Migrated to new Hashicorp Plugin SDK.
* Restore feature allows to restore server LUN's from snapshots.

BUG FIXES:
* Fixed provider crash issue in resourceDelete routine in case of `rancher_api` is not defined.

## 1.4.2 (November 24, 2020)

FEATURES:
* **New Argument:** `ssh_node_bootdisk_resize_commands` - (Optional) To support boot disk resize on host.
* **New Argument:** `ssh_node_datadisk_resize_commands` - (Optional) To support data disk resize on host.
* **New Argument:** `node_grace_timeout` - (Optional) Grace timeout after each node update in changing blade_spec or os_image/seed_template.

ENHANCEMENTS:
* Resource update routine now supports re-sizing for boot_lun and data_lun.

BUG FIXES:

## 1.4.1 (November 20, 2020)

FEATURES:
* **New Argument:** `rancher_api` - (Optional) Integration with Rancher API helps with node management of Rancher custom clusters.
* **New Argument:** `synchronized_updates` - (Optional) Forces sequential order for node updates.
* **New Argument:** `ssh_node_init_commands` - (Optional) Brings `provisioner` functionality inside `flexbot_server` resource for better error management and node updates functionality.

ENHANCEMENTS:
* Support for Rancher API which helps with graceful node management (cordon/drain/uncordon) in Rancher custom clusters.
* Support for synchronized node updates. Highly recommended for Rancher cluster node management.
* Improved node update routine.
* Added support for image and cloud-init seed templates updates

BUG FIXES:
* Fixed the bug with storage cleanup while cloud-init seed template updates
