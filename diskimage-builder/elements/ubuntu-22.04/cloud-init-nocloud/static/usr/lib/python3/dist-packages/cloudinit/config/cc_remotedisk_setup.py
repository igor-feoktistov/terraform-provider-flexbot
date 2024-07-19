# vi: ts=4 expandtab
#
#    Author: Igor Feoktistov <igorf@netapp.com>
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
############################################################################
MODULE_DESCRIPTION = """\
The module remotedisk_setup provides a simple and uniform way
to handle remote disks such as:
	- iSCSI LUN's:
	    - configures Open iSCSI initiator;
	    - configures device multipath;
	    - enables necessary services;
	    - attaches iSCSI LUN;
	    - discovers multipath device;
	    - creates logical volume;
	    - creates filesystem;
	    - mounts filesystem;
	    - configures /etc/fstab
	- NVME over TCP disks:
	    - configures host NQN;
	    - configures parameters for discovery;
	    - connects NVME device;
	    - creates filesystem;
	    - mounts filesystem;
	    - configures /etc/fstab
	- Hypervisor disks (OpenStack Cinder volumes, AWS EBS, etc):
	    - creates logical volume;
	    - creates filesystem;
	    - mounts filesystem;
	    - configures /etc/fstab
	- NFS shares:
	    - mounts NFS share;
	    - configures /etc/fstab
##########################################################
# Example configuration to handle iSCSI LUN:
##########################################################
remotedisk_setup:
   - device: 'iscsi:192.168.1.1:6:3260:1:iqn.1992-08.com.netapp:sn.62546b567fbf11e4811590e2ba6cc3b4:vs.10'
     lvm_group: 'vg_data1'
     lvm_volume: 'lv_data1'
     fs_type: 'xfs'
     mount_point: '/apps/data1'
##########################################################
# Parameters:
#    mandatory:
#       device: 'iscsi:<iSCSI target host/LIF>:<transport protocol>:<port>:<LUN ID>:<iSCSI target name>'
#       fs_type: '<filesystem type>'
#       mount_point: '<mount point dir path>'
#    optional:
#       initiator_name: '<iSCSI initiator name, default is iqn.2005-02.com.open-iscsi:<hostname>>'
#       mount_opts: '<filesystem mount options, default is "defaults,_netdev">'
#       lvm_group: '<LVM logical group name>'
#       lvm_volume: '<LVM logical volume name>'
#       fs_opts: '<filesystem create options specific to mkfs.fs_type>'
#       fs_label: '<FS label to use as device in /etc/fstab>'
#       fs_freq: '<fstab fs freq, default is "1">'
#       fs_passno: '<fstab fs passno, default is "2">'
#    notes:
#       missing lvm_group and lvm_volume will cause filesystem creation on top of multipath device
##########################################################
# Example configuration to handle NVME over TCP disk:
##########################################################
remotedisk_setup:
   - device: 'nvme:/vol/k8s_node1_data/k8s_node1_data:192.168.10.1:192.168.10.32,192.168.11.1:192.168.11.32'
     fs_type: 'xfs'
     fs_label: datafs
     mount_point: /kubernetes
     mount_opts: defaults,noatime,nodiratime,_netdev
##########################################################
# Parameters:
#    mandatory:
#       device: 'nvme:<namespace path>:<host IP>:<target IP>,<host IP>:<target IP>,...'
#       fs_type: '<filesystem type>'
#       mount_point: '<mount point dir path>'
#    optional:
#       host_nqn: 'host NQN, default is /etc/nvme/hostnqn'
#       mount_opts: '<filesystem mount options, default is "defaults,_netdev">'
#       fs_opts: '<filesystem create options specific to mkfs.fs_type>'
#       fs_label: '<FS label to use as device in /etc/fstab>'
#       fs_freq: '<fstab fs freq, default is "1">'
#       fs_passno: '<fstab fs passno, default is "2">'
##########################################################
# Example configuration to handle OpenStack Cinder volume:
##########################################################
remotedisk_setup:
   - device: 'ebs0'
     lvm_group: 'vg_data1'
     lvm_volume: 'lv_data1'
     fs_type: 'ext4'
     mount_point: '/apps/data1'
##########################################################
# Parameters:
#    mandatory:
#       device: 'ebs<0-9> or block device path /dev/vd<b-z>'
#       fs_type: '<filesystem type>'
#       mount_point: '<mount point dir path>'
#    optional:
#       mount_opts: '<filesystem mount options, default is "defaults">'
#       lvm_group: '<LVM logical group name>'
#       lvm_volume: '<LVM logical volume name>'
#       fs_opts: '<filesystem create options specific to mkfs.fs_type>'
#       fs_freq: '<fstab fs freq, default is "1">'
#       fs_passno: '<fstab fs passno, default is "2">'
#    notes:
#       missing lvm_group and lvm_volume will cause filesystem creation on top of block device
##########################################################
# Example configuration to handle NFS shares:
##########################################################
remotedisk_setup:
   - device: 'nfs:192.168.1.1:/myshare'
     mount_point: '/apps/data1'
     mount_opts: 'tcp,rw,rsize=65536,wsize=65536'
##########################################################
# Parameters:
#    mandatory:
#       device: 'nfs:<NFS host>:<NFS share path>'
#       mount_point: '<mount point dir path>'
#    optional:
#       mount_opts: '<NFS share mount options, default is "defaults">'
#       fs_type: 'nfs'
#       fs_freq: '<fstab fs freq, default is "0">'
#       fs_passno: '<fstab fs passno, default is "0">'
##########################################################
"""

import logging
import os
import time
import shlex
import fnmatch
import subprocess
import re
import json
import uuid
from string import whitespace

from cloudinit.cloud import Cloud
from cloudinit.config import Config
from cloudinit.config.schema import MetaSchema
from cloudinit.distros import ALL_DISTROS
from cloudinit.settings import PER_ALWAYS

from cloudinit.settings import PER_INSTANCE
from cloudinit import type_utils
from cloudinit import util
from cloudinit import subp
from cloudinit import templater

LANG_C_ENV = {"LANG": "C"}
LOG = logging.getLogger(__name__)

frequency = PER_INSTANCE

WAIT_4_BLOCKDEV_MAPPING_ITER = 60
WAIT_4_BLOCKDEV_MAPPING_SLEEP = 5
WAIT_4_BLOCKDEV_DEVICE_ITER = 12
WAIT_4_BLOCKDEV_DEVICE_SLEEP = 5

LVM_CMD = subp.which("lvm")
ISCSIADM_CMD = subp.which("iscsiadm")
NVME_CMD = subp.which("nvme")
MULTIPATH_CMD = subp.which("multipath")
SYSTEMCTL_CMD = subp.which("systemctl")
FSTAB_PATH = "/etc/fstab"
ISCSI_INITIATOR_PATH = "/etc/iscsi/initiatorname.iscsi"
NVME_HOSTNQN_PATH = "/etc/nvme/hostnqn"
NVME_HOSTID_PATH = "/etc/nvme/hostid"
NVME_DISCOVERY_CONF_PATH = "/etc/nvme/discovery.conf"

meta: MetaSchema = {
    "id": "cc_remotedisk_setup",
    "name": "Remote Disk Setup",
    "title": "remotedisk_setup provides a simple and uniform way to handle remote disks",
    "description": MODULE_DESCRIPTION,
    "distros": [ALL_DISTROS],
    "frequency": PER_ALWAYS,
    "examples": [],
}

# This module is undocumented in our schema docs
__doc__ = ""


def handle(name: str, cfg: Config, cloud: Cloud, args: list) -> None:
    if "remotedisk_setup" not in cfg:
        LOG.debug("Skipping module named %s, no configuration found" % _name)
        return
    remotedisk_setup = cfg.get("remotedisk_setup")
    LOG.debug("setting up remote disk: %s", str(remotedisk_setup))
    dev_entry_iscsi = 0
    for definition in remotedisk_setup:
        try:
            device = definition.get("device")
            if device:
                if device.startswith("iscsi"):
                    handle_iscsi(cfg, cloud, definition, dev_entry_iscsi)
                    dev_entry_iscsi += 1
                elif device.startswith("nvme"):
                    handle_nvme(cfg, cloud, definition)
                elif device.startswith("nfs"):
                    handle_nfs(cfg, cloud, definition)
                elif device.startswith("ebs"):
                    handle_ebs(cfg, cloud, definition)
                elif device.startswith("ephemeral"):
                    handle_ebs(cfg, cloud, definition)
                else:
                    if "fs_type" in definition:
                        fs_type = definition.get("fs_type")
                        if fs_type == "nfs":
                            handle_nfs(cfg, cloud, definition)
                        else:
                            handle_ebs(cfg, cloud, definition)
                    else:
                        util.logexc(LOG, "Expexted \"fs_type\" parameter")
            else:
                util.logexc(LOG, "Expexted \"device\" parameter")
        except Exception as e:
            util.logexc(LOG, "Failed during remote disk operation\n"
                             "Exception: %s" % e)


def handle_iscsi(cfg, cloud, definition, dev_entry_iscsi):
    # Handle iSCSI LUN
    device = definition.get("device")
    try:
        (iscsi_host,
         iscsi_proto,
         iscsi_port,
         iscsi_lun,
         iscsi_target) = device.split(":", 5)[1:]
    except Exception as e:
        util.logexc(LOG,
                    "handle_iscsi: "
                    "expected \"device\" attribute in the format: "
                    "\"iscsi:<iSCSI host>:<protocol>:<port>:<LUN>:"
                    "<iSCSI target name>\": %s" % e)
        return
    if dev_entry_iscsi == 0:
        (hostname, fqdn, is_default) = util.get_hostname_fqdn(cfg, cloud)
        if "initiator_name" in definition:
            initiator_name = definition.get("initiator_name")
        else:
            initiator_name = "iqn.2005-02.com.open-iscsi:%s" % hostname
        util.write_file(ISCSI_INITIATOR_PATH, "InitiatorName=%s" % initiator_name)
        multipath_tmpl_fn = cloud.get_template_filename("multipath.conf")
        if multipath_tmpl_fn:
            templater.render_to_file(multipath_tmpl_fn, "/etc/multipath.conf", {})
        else:
            LOG.warn("handle_iscsi: template multipath.conf not found")
        if cloud.distro.osfamily == "redhat":
            iscsi_services = ["iscsi", "iscsid"]
            multipath_services = ["multipathd"]
        elif cloud.distro.osfamily == 'debian':
            iscsi_services = ["open-iscsi", "iscsid"]
            multipath_services = ["multipathd"]
        else:
            util.logexc(LOG,
                        "handle_iscsi: "
                        "unsupported osfamily \"%s\"" % cloud.distro.osfamily)
            return
        for service in iscsi_services:
            _service_wrapper(cloud, service, "enable")
            _service_wrapper(cloud, service, "restart")
        for service in multipath_services:
            _service_wrapper(cloud, service, "enable")
            _service_wrapper(cloud, service, "restart")
    blockdev = _iscsi_lun_discover(iscsi_host,
                                   iscsi_port,
                                   iscsi_lun,
                                   iscsi_target)
    if blockdev:
        lvm_group = definition.get("lvm_group")
        lvm_volume = definition.get("lvm_volume")
        fs_type = definition.get("fs_type")
        fs_label = definition.get("fs_label")
        fs_opts = definition.get("fs_opts")
        mount_point = definition.get("mount_point")
        mount_opts = definition.get("mount_opts")
        if not mount_opts:
            mount_opts = 'defaults,_netdev'
        else:
            if mount_opts.find("_netdev") == -1:
                mount_opts = "%s,_netdev" % (mount_opts)
        fs_freq = definition.get("fs_freq")
        if not fs_freq:
            fs_freq = "1"
        fs_passno = definition.get("fs_passno")
        if not fs_passno:
            fs_passno = "2"
        if lvm_group and lvm_volume:
            for vg_name in _list_vg_names():
                if vg_name == lvm_group:
                    util.logexc(LOG,
                                "handle_iscsi: "
                                "logical volume group '%s' exists already"
                                % lvm_group)
                    return
            for lv_name in _list_lv_names():
                if lv_name == lvm_volume:
                    util.logexc(LOG,
                                "handle_iscsi: "
                                "logical volume '%s' exists already"
                                % lvm_volume)
                    return
            blockdev = _create_lv(blockdev, lvm_group, lvm_volume)
        if blockdev:
            if mount_point and fs_type:
                _create_fs(blockdev, fs_type, fs_label, fs_opts)
                _add_fstab_entry(blockdev,
                                 mount_point,
                                 fs_type,
                                 fs_label,
                                 mount_opts,
                                 fs_freq,
                                 fs_passno)
                _mount_fs(mount_point)
            else:
                util.logexc(LOG,
                            "handle_iscsi: "
                            "expexted \"mount_point\" "
                            "and \"fs_type\" parameters")


def handle_nvme(cfg, cloud, definition):
    # Handle NVME over TCP disk
    device = definition.get("device")
    try:
        (proto, namespace, host_list) = device.split(":", 2)
        nvme_hosts = host_list.split(",")
    except Exception as e:
        util.logexc(LOG,
                    "handle_nvme: "
                    "expected \"device\" attribute in the format: "
                    "\"nvme:<namespace path><host IP>:<target IP>,<host IP>:<target IP>,...\": %s" % e)
        return
    if len(nvme_hosts) == 0:
        util.logexc(LOG,
                    "handle_nvme: "
                    "expected \"device\" attribute in the format: "
                    "\"nvme:<namespace path>:<host IP>:<target IP>:<host IP>:<target IP>,...\": %s" % e)
        return
    if "host_nqn" in definition:
        host_nqn = "%s\n" % definition.get("host_nqn")
        util.write_file(NVME_HOSTNQN_PATH, host_nqn)
        host_id = "%s\n" % str(uuid.uuid4())
        util.write_file(NVME_HOSTID_PATH, host_id)
    discovery_lines = []
    for nvme_host in nvme_hosts:
        (host_ip, target_ip) = nvme_host.split(":")
        discovery_lines.append("--transport=tcp --host-traddr=%s --traddr=%s" % (host_ip, target_ip))
    discovery_contents = "%s\n" % ('\n'.join(discovery_lines))
    util.write_file(NVME_DISCOVERY_CONF_PATH, discovery_contents)
    blockdev = _nvme_discover(namespace)
    if blockdev:
        fs_type = definition.get("fs_type")
        fs_label = definition.get("fs_label")
        fs_opts = definition.get("fs_opts")
        mount_point = definition.get("mount_point")
        mount_opts = definition.get("mount_opts")
        if not mount_opts:
            mount_opts = 'defaults,_netdev'
        else:
            if mount_opts.find("_netdev") == -1:
                mount_opts = "%s,_netdev" % (mount_opts)
        fs_freq = definition.get("fs_freq")
        if not fs_freq:
            fs_freq = "1"
        fs_passno = definition.get("fs_passno")
        if not fs_passno:
            fs_passno = "2"
        if mount_point and fs_type:
            _create_fs(blockdev, fs_type, fs_label, fs_opts)
            _add_fstab_entry(blockdev,
                            mount_point,
                            fs_type,
                            fs_label,
                            mount_opts,
                            fs_freq,
                            fs_passno)
            _mount_fs(mount_point)
        else:
            util.logexc(LOG,
                        "handle_nvme: "
                        "expexted \"mount_point\" "
                        "and \"fs_type\" parameters")


def handle_nfs(cfg, cloud, definition):
    # Handle NFS share mounts
    device = definition.get("device")
    if device.startswith("nfs"):
        (proto, share_path) = device.split(":", 1)
    else:
        share_path = device
    fs_type = definition.get("fs_type")
    mount_point = definition.get("mount_point")
    mount_opts = definition.get("mount_opts")
    if not mount_opts:
        mount_opts = "defaults"
    fs_freq = definition.get("fs_freq")
    if not fs_freq:
        fs_freq = "0"
    fs_passno = definition.get("fs_passno")
    if not fs_passno:
        fs_passno = "0"
    if mount_point and fs_type:
        _add_fstab_entry(share_path,
                         mount_point,
                         fs_type,
                         None,
                         mount_opts,
                         fs_freq,
                         fs_passno)
        _mount_fs(mount_point)
    else:
        util.logexc(LOG,
                    "handle_nfs: "
                    "expexted \"mount_point\" and \"fs_type\" parameters")


def handle_ebs(cfg, cloud, definition):
    # Handle block device either explicitly provided via device path or
    # via device name mapping (Amazon/OpenStack)
    device = definition.get("device")
    blockdev = _cloud_device_2_os_device(cloud, device)
    if blockdev:
        lvm_group = definition.get("lvm_group")
        lvm_volume = definition.get("lvm_volume")
        fs_type = definition.get("fs_type")
        fs_label = definition.get("fs_label")
        fs_opts = definition.get("fs_opts")
        mount_point = definition.get("mount_point")
        mount_opts = definition.get("mount_opts")
        if not mount_opts:
            mount_opts = "defaults"
        fs_freq = definition.get("fs_freq")
        if not fs_freq:
            fs_freq = "1"
        fs_passno = definition.get("fs_passno")
        if not fs_passno:
            fs_passno = "2"
        if lvm_group and lvm_volume:
            for vg_name in _list_vg_names():
                if vg_name == lvm_group:
                    util.logexc(LOG,
                                "handle_ebs: "
                                "logical volume group '%s' exists already"
                                % lvm_group)
                    return
            for lv_name in _list_lv_names():
                if lv_name == lvm_volume:
                    util.logexc(LOG,
                                "handle_ebs: "
                                "logical volume '%s' exists already"
                                % lvm_volume)
                    return
            blockdev = _create_lv(blockdev, lvm_group, lvm_volume)
        if blockdev:
            if mount_point and fs_type:
                _create_fs(blockdev, fs_type, fs_label, fs_opts)
                _add_fstab_entry(blockdev,
                                 mount_point,
                                 fs_type,
                                 fs_label,
                                 mount_opts,
                                 fs_freq,
                                 fs_passno)
                _mount_fs(mount_point)
            else:
                util.logexc(LOG,
                            "handle_ebs: "
                            "expexted \"mount_point\" and "
                            "\"fs_type\" parameters")


def _cloud_device_2_os_device(cloud, name):
    # Translate cloud device (ebs# and ephemaral#) to OS block device path
    blockdev = None
    for i in range(WAIT_4_BLOCKDEV_MAPPING_ITER):
        if (cloud.datasource.metadata and
                "block-device-mapping" in cloud.datasource.metadata):
            metadata = cloud.datasource.metadata
        else:
            if (cloud.datasource.ec2_metadata and
                    "block-device-mapping" in cloud.datasource.ec2_metadata):
                metadata = cloud.datasource.ec2_metadata
            else:
                util.logexc(LOG,
                            "_cloud_device_2_os_device: "
                            "metadata item block-device-mapping not found")
                return None
        blockdev_items = metadata["block-device-mapping"].iteritems()
        for (map_name, device) in blockdev_items:
            if map_name == name:
                blockdev = device
                break
        if blockdev is None:
            cloud.datasource.get_data()
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
            continue
    if blockdev is None:
        util.logexc(LOG,
                    "_cloud_device_2_os_device: "
                    "unable to convert %s to a device" % name)
        return None
    if not blockdev.startswith("/"):
        blockdev_path = "/dev/%s" % blockdev
    else:
        blockdev_path = blockdev
    for i in range(WAIT_4_BLOCKDEV_DEVICE_ITER):
        if os.path.exists(blockdev_path):
            return blockdev_path
        time.sleep(WAIT_4_BLOCKDEV_DEVICE_SLEEP)
    util.logexc(LOG,
                "_cloud_device_2_os_device: "
                "device %s does not exist" % blockdev_path)
    return None


def _list_vg_names():
    # List all LVM volume groups
    p = subprocess.Popen([LVM_CMD, "vgs", "-o", "vg_name"],
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    err = p.wait()
    if err:
        return []
    output = p.communicate()[0]
    output = output.decode().split("\n")
    if not output:
        return []
    header = output[0].strip()
    if header != "VG":
        return []
    names = []
    for name in output[1:]:
        if not name:
            break
        names.append(name.strip())
    return names


def _list_lv_names():
    # List all LVM logical volumes
    p = subprocess.Popen([LVM_CMD, "lvs", "-o", "lv_name"],
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    err = p.wait()
    if err:
        return []
    output = p.communicate()[0]
    output = output.decode().split("\n")
    if not output:
        return []
    header = output[0].strip()
    if header != "LV":
        return []
    names = []
    for name in output[1:]:
        if not name:
            break
        names.append(name.strip())
    return names


def _create_lv(device, vg_name, lv_name):
    # Create volume group
    pvcreate_cmd = [LVM_CMD, "pvcreate", device]
    vgcreate_cmd = [LVM_CMD, "vgcreate", "-f", vg_name, device]
    lvcreate_cmd = [LVM_CMD,
                    "lvcreate", "-l", "100%FREE", "--name", lv_name, vg_name]
    try:
        subp.subp(pvcreate_cmd)
        subp.subp(vgcreate_cmd)
        subp.subp(lvcreate_cmd)
        return "/dev/mapper/%s-%s" % (vg_name, lv_name)
    except Exception as e:
        util.logexc(LOG,
                    "_create_lv: "
                    "failed to create LVM volume '%s' for device '%s': %s"
                    % (lv_name, device, e))
        return None


def _create_fs(device, fs_type, fs_label, fs_opts=None):
    # Create filesystem
    mkfs_cmd = subp.which("mkfs.%s" % fs_type)
    if not mkfs_cmd:
        mkfs_cmd = subp.which("mk%s" % fs_type)
    if not mkfs_cmd:
        util.logexc(LOG,
                    "_create_fs: "
                    "cannot create filesystem type '%s': "
                    "failed to find mkfs.%s command" % (fs_type, fs_type))
        return
    try:
        if fs_opts:
            if fs_label:
                subp.subp([mkfs_cmd, '-L', fs_label, fs_opts, device])
            else:
                subp.subp([mkfs_cmd, fs_opts, device])
        else:
            if fs_label:
                subp.subp([mkfs_cmd, '-L', fs_label, device])
            else:
                subp.subp([mkfs_cmd, device])
    except Exception as e:
        util.logexc(LOG,
                    "_create_fs: "
                    "failed to create filesystem type '%s': %s" % (fs_type, e))


def _add_fstab_entry(device,
                     mount_point,
                     fs_type,
                     fs_label,
                     mount_opts,
                     fs_freq,
                     fs_passno):
    # Create fstab entry
    fstab_lines = []
    for line in util.load_text_file(FSTAB_PATH).splitlines():
        try:
            toks = re.compile("[%s]+" % (whitespace)).split(line)
        except:
            pass
        if len(toks) > 0 and toks[0] == device:
            util.logexc(LOG,
                        "_add_fstab_entry: "
                        "file %s has device %s already" % (FSTAB_PATH, device))
            return
        if len(toks) > 1 and toks[1] == mount_point:
            util.logexc(LOG,
                        "_add_fstab_entry: "
                        "file %s has mount point %s already"
                        % (FSTAB_PATH, mount_point))
            return
        fstab_lines.append(line)
    if fs_label:
        device = "LABEL=%s" % fs_label
    fstab_lines.extend(["%s\t%s\t%s\t%s\t%s\t%s" %
                       (device,
                        mount_point,
                        fs_type,
                        mount_opts,
                        fs_freq,
                        fs_passno)])
    contents = "%s\n" % ('\n'.join(fstab_lines))
    util.write_file(FSTAB_PATH, contents)


def _mount_fs(mount_point):
    # Mount filesystem according to fstab entry
    try:
        util.ensure_dir(mount_point)
    except Exception as e:
        util.logexc(LOG,
                    "_mount_fs: "
                    "failed to make '%s' mount point directory: %s"
                    % (mount_point, e))
        return
    try:
        subp.subp(["mount", mount_point])
    except Exception as e:
        util.logexc(LOG,
                    "_mount_fs: "
                    "activating mounts via 'mount %s' failed: %s"
                    % (mount_point, e))


def _service_wrapper(cloud, service, command):
    # Wrapper for service related commands
    if cloud.distro.osfamily == "redhat":
        svc_cmd = [SYSTEMCTL_CMD, command, service]
    elif cloud.distro.osfamily == "debian":
        svc_cmd = [SYSTEMCTL_CMD, command, service]
    else:
        util.logexc(LOG,
                    "_handle_service: "
                    "unsupported osfamily \"%s\"" % cloud.distro.osfamily)
        return
    try:
        subp.subp(svc_cmd, capture=False)
    except Exception as e:
        util.logexc(LOG,
                    "_handle_service: "
                    "failure to \"%s\" \"%s\": %s" % (command, service, e))


def _iscsi_lun_discover(iscsi_host, iscsi_port, iscsi_lun, iscsi_target):
    # Discover iSCSI target and map LUN ID to multipath device path
    blockdev = None
    mpathdev = None
    for i in range(WAIT_4_BLOCKDEV_MAPPING_ITER):
        try:
            subp.subp([ISCSIADM_CMD,
                       "--mode",
                       "discoverydb",
                       "--type",
                       "sendtargets",
                       "--portal",
                       "%s:%s" % (iscsi_host, iscsi_port),
                       "--discover",
                       "--login",
                       "all"],
                      capture=False)
        except Exception as e:
            pass
        p = subprocess.Popen([ISCSIADM_CMD, "-m", "node"],
                             stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
        err = p.wait()
        if err:
            util.logexc(LOG,
                        "_iscsi_lun_discover: "
                        "failure from \"%s -m node\" command" % ISCSIADM_CMD)
            return None
        output = p.communicate()[0]
        output = output.decode().split("\n")
        if not output:
            util.logexc(LOG,
                        "_iscsi_lun_discover: "
                        "no iSCSI nodes discovered for target \"%s\""
                        % iscsi_target)
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
            continue
        for node in output:
            iscsi_portal = node.split(",", 1)[0]
            if iscsi_portal:
                try:
                    subp.subp([ISCSIADM_CMD,
                               "-m",
                               "node",
                               "-T",
                               iscsi_target,
                               "-p",
                               iscsi_portal,
                               "--op",
                               "update",
                               "-n",
                               "node.startup",
                               "-v",
                               "automatic"],
                              capture=False)
                except Exception as e:
                    util.logexc(LOG,
                                "_iscsi_lun_discover: "
                                "failure in attempt to set automatic binding "
                                "for target portal \"%s\": %s"
                                % (iscsi_portal, e))
                    pass
        p = subprocess.Popen([ISCSIADM_CMD, "-m", "session", "-P3"],
                             stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
        err = p.wait()
        if err:
            util.logexc(LOG,
                        "_iscsi_lun_discover: "
                        "failure from \"%s -m session -P3\" command"
                        % ISCSIADM_CMD)
            return None
        output = p.communicate()[0]
        output = output.decode().split("\n")
        if not output:
            util.logexc(LOG,
                        "_iscsi_lun_discover: "
                        "no iSCSI sessions discovered for target \"%s\""
                        % iscsi_target)
        else:
            current_iscsi_target = None
            current_iscsi_sid = None
            current_iscsi_lun = None
            for line in output:
                m = re.search("^Target: ([a-z0-9\.:-]*)", line)
                if m:
                    current_iscsi_target = m.group(1)
                    continue
                else:
                    if (current_iscsi_target and
                            current_iscsi_target == iscsi_target):
                        m = re.search("SID: ([0-9]*)", line)
                        if m:
                            if current_iscsi_sid and not current_iscsi_lun:
                                try:
                                    subp.subp([ISCSIADM_CMD,
                                               "-m",
                                               "session",
                                               "-r",
                                               current_iscsi_sid,
                                               "-u"],
                                              capture=False)
                                except:
                                    pass
                            current_iscsi_sid = m.group(1)
                            current_iscsi_lun = None
                            continue
                        m = re.search("scsi[0-9]* Channel [0-9]* "
                                      "Id [0-9]* Lun: ([0-9]*)", line)
                        if m:
                            current_iscsi_lun = m.group(1)
                            continue
                        if (current_iscsi_lun and
                                current_iscsi_lun == iscsi_lun):
                            m = re.search("Attached scsi disk (sd[a-z]*)",
                                          line)
                            if m:
                                attached_scsi_disk = m.group(1)
                                p = subprocess.Popen(["/lib/udev/scsi_id",
                                                      "-g", "-u", "-d",
                                                      "/dev/%s"
                                                      % attached_scsi_disk],
                                                     stdout=subprocess.PIPE,
                                                     stderr=subprocess.PIPE)
                                err = p.wait()
                                if err:
                                    util.logexc(LOG,
                                                "_iscsi_lun_discover: "
                                                "failure from "
                                                "\"/lib/udev/scsi_id\" "
                                                "command")
                                    return None
                                output2 = p.communicate()[0]
                                output2 = output2.decode().split('\n')
                                if not output2:
                                    util.logexc(LOG,
                                                "_iscsi_lun_discover: "
                                                "no wwid returned for device "
                                                "\"/dev/%s\""
                                                % attached_scsi_disk)
                                else:
                                    mpathdev = output2[0]
                                    blockdev = "/dev/mapper/%s" % output2[0]
            if current_iscsi_sid and not current_iscsi_lun:
                try:
                    subp.subp([ISCSIADM_CMD,
                               "-m",
                               "session",
                               "-r",
                               current_iscsi_sid,
                               "-u"],
                              capture=False)
                except:
                    pass
        if blockdev:
            break
        else:
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
    if blockdev:
        try:
            subp.subp([MULTIPATH_CMD, "-f", mpathdev], capture=False)
        except Exception as e:
            pass
        for i in range(WAIT_4_BLOCKDEV_DEVICE_ITER):
            if os.path.exists(blockdev):
                return blockdev
            try:
                subp.subp([MULTIPATH_CMD, mpathdev], capture=False)
            except Exception as e:
                util.logexc(LOG,
                            "_iscsi_lun_discover: "
                            "failure to run \"%s\": %s" % (MULTIPATH_CMD, e))
                return None
            time.sleep(WAIT_4_BLOCKDEV_DEVICE_SLEEP)
    else:
        return None


def _nvme_discover(namespace):
    # Discover NVME target for given NMVE namespace path and return device path
    try:
        subp.subp([NVME_CMD, "connect-all"], capture=False)
    except Exception as e:
        util.logexc(LOG,
                    "_nvme_discover: "
                    "failure from \"%s connect-all\" command: %s" % NVME_CMD, e)
        return None
    p_output = None
    try:
        p = subprocess.Popen([NVME_CMD, "netapp", "ontapdevices", "-o" "json"],
                             stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
        err = p.wait()
        if err:
            util.logexc(LOG,
                        "_nvme_discover: "
                        "failure from \"%s netapp ontapdevices -o json\" command" % NVME_CMD)
            return None
        p_output = p.communicate()[0]
    except Exception as e:
        util.logexc(LOG, "_nvme_discover: exception: %s" % e)
        return None
    for i in range(WAIT_4_BLOCKDEV_DEVICE_ITER):
        nvme_devices = None
        try:
            nvme_devices = json.loads(p_output)
            for dev in nvme_devices["ONTAPdevices"]:
                if dev["Namespace_Path"] == namespace:
                    return dev["Device"]
        except Exception as e:
            pass
        time.sleep(WAIT_4_BLOCKDEV_DEVICE_SLEEP)
    util.logexc(LOG, "_nvme_discover: no devices found")
    return None
