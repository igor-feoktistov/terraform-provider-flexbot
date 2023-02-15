package flexbot

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/crypto/ssh"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ipam"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ucsm"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/rancher"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	log "github.com/sirupsen/logrus"
)

// Default timeouts
const (
	NodeRestartTimeout = 600
	NodeGraceShutdownTimeout = 60
	Wait4ClusterTransitioningTimeout = 120
)

// Change status definition while update routine
const (
	ChangeBladeSpec       = 1
	ChangePowerState      = 2
	ChangeSnapshotCreate  = 4
	ChangeSnapshotDelete  = 8
	ChangeSnapshotRestore = 16
	ChangeOsImage         = 32
	ChangeSeedTemplate    = 64
	ChangeBootDiskSize    = 128
	ChangeDataDiskSize    = 256
)

func resourceFlexbotServer() *schema.Resource {
	return &schema.Resource{
		Schema:        schemaFlexbotServer(),
		CreateContext: resourceCreateServer,
		ReadContext:   resourceReadServer,
		UpdateContext: resourceUpdateServer,
		DeleteContext: resourceDeleteServer,
		Importer: &schema.ResourceImporter{
			StateContext: resourceImportServer,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(7200 * time.Second),
			Update: schema.DefaultTimeout(28800 * time.Second),
			Delete: schema.DefaultTimeout(1800 * time.Second),
		},
	}
}

func resourceCreateServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	var sshPrivateKey string
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		diags = diag.FromErr(err)
		return
	}
	for _, snapshot := range d.Get("snapshot").([]interface{}) {
		name := snapshot.(map[string]interface{})["name"].(string)
		if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
			if len(sshUser) == 0 || len(sshPrivateKey) == 0 {
				diags = diag.FromErr(fmt.Errorf("resourceCreateServer(): expected compute.ssh_user and compute.ssh_private_key parameters to ensure fsfreeze for snapshot %s", name))
				return
			}
		}
	}
	log.Infof("Creating Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if serverExists {
		diags = diag.FromErr(fmt.Errorf("resourceCreateServer(): serverServer %s already exists", nodeConfig.Compute.HostName))
		return
	}
	var ipamProvider ipam.IpamProvider
	if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if err = ipamProvider.AllocatePreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ipamProvider.AllocatePreflight()",
			Detail:   err.Error(),
		})
	}
	if err = ontap.CreateBootStoragePreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.CreateBootStoragePreflight()",
			Detail:   err.Error(),
		})
	}
	if err = ucsm.CreateServerPreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ucsm.CreateServerPreflight()",
			Detail:   err.Error(),
		})
	}
	if err = ontap.CreateSeedStoragePreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.CreateSeedStoragePreflight()",
			Detail:   err.Error(),
		})
	}
	if len(diags) > 0 {
		return
	}
	if err = ipamProvider.Allocate(nodeConfig); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceCreateServer(): %s", err))
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.SetId(nodeConfig.Compute.HostName)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	err = ontap.CreateBootStorage(nodeConfig)
	if err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		err = ontap.CreateNvmeStorage(nodeConfig)
	}
	if err == nil {
		err = ontap.CreateSeedStorage(nodeConfig)
	}
	if err == nil {
		err = ucsm.StartServer(nodeConfig)
	}
	if err == nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
		d.SetConnInfo(map[string]string{"type": "ssh", "host": nodeConfig.Network.Node[0].Ip})
                meta.(*config.FlexbotConfig).Sync.Unlock()
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 && err == nil {
		if err = waitForSSH(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
			for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
				var cmdOutput string
				log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					break
				}
				if len(cmdOutput) > 0 && log.IsLevelEnabled(log.DebugLevel) {
					log.Debugf("Completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
	}
	if err == nil {
		for _, snapshot := range d.Get("snapshot").([]interface{}) {
			name := snapshot.(map[string]interface{})["name"].(string)
			if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
				err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
			} else {
				err = ontap.CreateSnapshot(nodeConfig, name, "")
			}
			if err != nil {
				break
			}
		}
	}
	setFlexbotOutput(d, meta, nodeConfig)
	if err == nil {
		var rancherNode rancher.RancherNode
		if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true); err == nil {
			err = rancherNode.RancherAPINodeSetAnnotationsLabelsTaints()
		}
	}
	if err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceCreateServer(): %s", err))
	}
	return
}

func resourceReadServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Refreshing Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	var storageExists bool
	if storageExists, err = ontap.DiscoverBootStorage(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if serverExists && storageExists {
		var ipamProvider ipam.IpamProvider
		if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
			diags = diag.FromErr(err)
			return
		}
		if err = ipamProvider.Discover(nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
		if err = rancher.DiscoverNode(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
		setFlexbotOutput(d, meta, nodeConfig)
	} else {
                meta.(*config.FlexbotConfig).Sync.Lock()
		d.SetId("")
                meta.(*config.FlexbotConfig).Sync.Unlock()
	}
	return
}

func resourceUpdateServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	var isNew, isSnapshot, isCompute, isStorage, isLabels, isTaints, isRestore, isMaintenance bool
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	isNew = d.IsNewResource()
	isSnapshot = d.HasChange("snapshot")
        isCompute = d.HasChange("compute")
        isStorage = d.HasChange("storage")
        isLabels = d.HasChange("labels")
        isTaints = d.HasChange("taints")
        isRestore = d.HasChange("restore")
        isMaintenance = d.HasChange("maintenance")
        if isCompute || isStorage || isSnapshot || isRestore || isMaintenance {
	        d.Partial(true)
        }
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if isSnapshot && !isNew {
		if err = resourceUpdateServerSnapshot(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if isCompute && !isNew {
		if err = resourceUpdateServerCompute(d, meta, nodeConfig); err != nil {
			resourceReadServer(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if isStorage && !isNew {
		if err = resourceUpdateServerStorage(d, meta, nodeConfig); err != nil {
			resourceReadServer(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if isLabels {
		if err = resourceUpdateServerLabels(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if isTaints {
		if err = resourceUpdateServerTaints(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if isMaintenance {
		if err = resourceUpdateServerMaintenance(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if isRestore {
		if err = resourceUpdateServerRestore(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	resourceReadServer(ctx, d, meta)
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.Partial(false)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if (nodeConfig.ChangeStatus & (ChangeBladeSpec | ChangeOsImage | ChangeSeedTemplate | ChangeSnapshotRestore)) > 0 {
		var rancherNode rancher.RancherNode
		if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, (nodeConfig.Compute.Powerstate == "up")); err == nil {
			if err = rancherNode.RancherAPINodeSetAnnotationsLabelsTaints(); err != nil {
				diags = diag.FromErr(err)
			}
		} else {
			diags = diag.FromErr(err)
		}
	}
	return
}

func resourceUpdateServerCompute(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, newPowerState, sshPrivateKey string
	var oldBladeSpec, newBladeSpec map[string]interface{}
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldCompute, newCompute := d.GetChange("compute")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(compute): failure: %s", err)
		return
	}
	if len((oldCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})) > 0 {
		oldBladeSpec = (oldCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
	} else {
		oldBladeSpec = make(map[string]interface{})
		oldBladeSpec["dn"] = nodeConfig.Compute.BladeAssigned.Dn
		oldBladeSpec["model"] = nodeConfig.Compute.BladeAssigned.Model
		oldBladeSpec["num_of_cpus"] = nodeConfig.Compute.BladeAssigned.NumOfCpus
		oldBladeSpec["num_of_cores"] = nodeConfig.Compute.BladeAssigned.NumOfCores
		oldBladeSpec["num_of_threads"] = nodeConfig.Compute.BladeAssigned.NumOfThreads
		oldBladeSpec["total_memory"] = nodeConfig.Compute.BladeAssigned.TotalMemory
	}
	if len((newCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})) > 0 {
		newBladeSpec = (newCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
		for _, specItem := range []string{"dn", "model"} {
			if oldBladeSpec[specItem].(string) != newBladeSpec[specItem].(string) && len(newBladeSpec[specItem].(string)) > 0 {
				var matched bool
				if matched, err = regexp.MatchString(newBladeSpec[specItem].(string), compute["blade_assigned"].([]interface{})[0].(map[string]interface{})[specItem].(string)); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute):  regexp.MatchString(%s), error: %s", newBladeSpec[specItem].(string), err)
					return
				}
				if !matched {
					nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeBladeSpec
				}
			}
		}
		for _, specItem := range []string{"num_of_cpus", "num_of_cores", "num_of_threads", "total_memory"} {
			if oldBladeSpec[specItem].(string) != newBladeSpec[specItem].(string) && len(newBladeSpec[specItem].(string)) > 0 {
				var inRange bool
				var specValue int
				if specValue, err = strconv.Atoi(compute["blade_assigned"].([]interface{})[0].(map[string]interface{})[specItem].(string)); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): unexpected value %s=%s, error: %s", specItem, compute["blade_assigned"].([]interface{})[0].(map[string]interface{})[specItem].(string), err)
					return
				}
				if inRange, err = valueInRange(newBladeSpec[specItem].(string), specValue); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): unexpected blade_spec value %s=%s, error: %s", specItem, newBladeSpec[specItem].(string), err)
					return
				}
				if !inRange {
					nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeBladeSpec
				}
			}
		}
	}
	if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
		nodeConfig.Compute.BladeSpec.Dn = newBladeSpec["dn"].(string)
	}
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	}
	nodeConfig.Compute.Powerstate = powerState
	newPowerState = (newCompute.([]interface{})[0].(map[string]interface{}))["powerstate"].(string)
	if newPowerState != powerState {
		nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangePowerState
	}
	if (nodeConfig.ChangeStatus & (ChangeBladeSpec | ChangePowerState)) > 0 {
		err = meta.(*config.FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*config.FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateServer(compute): last resource instance update returned error: %s", err)
			return
		}
		log.Infof("Updating Server Compute for node %s", nodeConfig.Compute.HostName)
	        if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
	                if err = ucsm.UpdateServerPreflight(nodeConfig); err != nil {
			        err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
			        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			        return
	                }
	        }
		var rancherNode rancher.RancherNode
		if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, false); err != nil {
			err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" {
			if (newCompute.([]interface{})[0].(map[string]interface{}))["safe_removal"].(bool) {
				err = fmt.Errorf("resourceUpdateServer(compute): server %s has power state up", nodeConfig.Compute.HostName)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			// Cordon/drain worker nodes
			if rancherNode.IsNodeWorker() {
				if err = rancherNode.RancherAPINodeCordonDrain(); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
                                if err = shutdownServer(nodeConfig, sshUser, sshPrivateKey); err != nil {
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        return
                                }
			}
		}
		if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
			if err = ucsm.UpdateServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if newPowerState == "up" {
			if err = ucsm.StartServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				if err = waitForSSH(nodeConfig, (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			// Uncordon worker nodes
			if rancherNode.IsNodeWorker() {
				if err = rancherNode.RancherAPINodeUncordon(); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout); err != nil {
				err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if meta.(*config.FlexbotConfig).NodeGraceTimeout > 0 {
			        if err = rancherNode.RancherAPINodeWaitForGracePeriod(meta.(*config.FlexbotConfig).NodeGraceTimeout); err != nil {
				        err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
				        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				        return
				}
			}
		}
	}
	if (oldCompute.([]interface{})[0].(map[string]interface{}))["description"].(string) != (newCompute.([]interface{})[0].(map[string]interface{}))["description"].(string) ||
		(oldCompute.([]interface{})[0].(map[string]interface{}))["label"].(string) != (newCompute.([]interface{})[0].(map[string]interface{}))["label"].(string) {
		err = ucsm.UpdateServerAttributes(nodeConfig)
	}
	return
}

func resourceUpdateServerStorage(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, sshPrivateKey string
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldStorage, newStorage := d.GetChange("storage")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(storage): failure: %s", err)
		return
	}
	oldBootLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	newBootLun := (newStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	oldSeedLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	newSeedLun := (newStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	if oldBootLun["os_image"].(string) != newBootLun["os_image"].(string) || oldSeedLun["seed_template"].(string) != newSeedLun["seed_template"].(string) || (newStorage.([]interface{})[0].(map[string]interface{}))["force_update"].(bool) {
		if oldBootLun["os_image"].(string) != newBootLun["os_image"].(string) || (newStorage.([]interface{})[0].(map[string]interface{}))["force_update"].(bool){
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeOsImage
		}
		if oldSeedLun["seed_template"].(string) != newSeedLun["seed_template"].(string) || (newStorage.([]interface{})[0].(map[string]interface{}))["force_update"].(bool) {
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSeedTemplate
		}
		log.Infof("Updating Server Storage image for node %s", nodeConfig.Compute.HostName)
		err = meta.(*config.FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*config.FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): last resource instance update returned error: %s", err)
			return
		}
		if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		nodeConfig.Storage.BootLun.OsImage.Name = newBootLun["os_image"].(string)
		nodeConfig.Storage.SeedLun.SeedTemplate.Location = newSeedLun["seed_template"].(string)
		log.Infof("Running boot storage preflight check")
		if err = ontap.CreateBootStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): boot storage preflight check error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		log.Infof("Running seed storage preflight check")
		if err = ontap.CreateSeedStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): seed storage preflight check error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" && compute["safe_removal"].(bool) {
			err = fmt.Errorf("resourceUpdateServer(storage): server %s has power state up", nodeConfig.Compute.HostName)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		var rancherNode rancher.RancherNode
		if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, false); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	        // Cordon/drain worker nodes
		if powerState == "up" && rancherNode.IsNodeWorker() {
		        if err = rancherNode.RancherAPINodeCordonDrain(); err != nil {
			        err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
                }
		// Delete etcd/controlplane node or RKE2 worker node
		if rancherNode.IsNodeEtcd() || rancherNode.IsNodeControlPlane() || rancherNode.IsProviderRKE2() {
			if err = rancherNode.RancherAPINodeDelete(); err == nil {
			        rancherNode.RancherAPIClusterWaitForTransitioning(Wait4ClusterTransitioningTimeout)
				err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout)
			}
			if err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if powerState == "up" {
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if (newStorage.([]interface{})[0].(map[string]interface{}))["auto_snapshot_on_update"].(bool) {
			t := time.Now()
			snapshotName := fmt.Sprintf("terraform:%s:%s-%s", oldBootLun["os_image"].(string), oldSeedLun["seed_template"].(string), t.Format(time.RFC3339))
			if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = ontap.DeleteBootLUNs(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateBootStorage(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateNvmeStorage(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateSeedStorage(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ucsm.StartServer(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
			if err = waitForSSH(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
				if err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout); err != nil {
					err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
					var cmdOutput string
					log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
					if cmdOutput, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
						meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
						return
					}
					if len(cmdOutput) > 0 && log.IsLevelEnabled(log.DebugLevel) {
						log.Debugf("Completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
					}
				}
			}
		}
		if rancherNode.IsNodeEtcd() || rancherNode.IsNodeControlPlane() || rancherNode.IsProviderRKE2() {
			rancherNode.RancherAPIClusterWaitForTransitioning(Wait4ClusterTransitioningTimeout)
		        if err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout); err == nil {
		                err = rancherNode.RancherAPINodeGetID(d, meta);
		        }
		        if err != nil {
			        err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			        return
		        }
		}
		// Uncordon worker nodes
		if rancherNode.IsNodeWorker() && rancherNode.IsProviderRKE1() {
		        if err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout); err == nil {
			        err = rancherNode.RancherAPINodeUncordon()
			}
			if err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = rancherNode.RancherAPINodeWaitForState("active", rancher.Wait4NodeStateTimeout); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if meta.(*config.FlexbotConfig).NodeGraceTimeout > 0 {
		        if err = rancherNode.RancherAPINodeWaitForGracePeriod(meta.(*config.FlexbotConfig).NodeGraceTimeout); err != nil {
			        err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
	}
	if oldBootLun["size"].(int) != newBootLun["size"].(int) {
		log.Infof("Re-sizing Server Storage boot LUN for node %s", nodeConfig.Compute.HostName)
		nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeBootDiskSize
	}
	if len((oldStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})) > 0 && len((newStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})) > 0 {
		oldDataLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})[0].(map[string]interface{})
		newDataLun := (newStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})[0].(map[string]interface{})
		if oldDataLun["size"].(int) != newDataLun["size"].(int) {
			log.Infof("Re-sizing Server Storage data LUN for node %s", nodeConfig.Compute.HostName)
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeDataDiskSize
		}
	}
	if (nodeConfig.ChangeStatus & (ChangeBootDiskSize | ChangeDataDiskSize)) > 0 {
		if err = ontap.ResizeBootStorage(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			return
		}
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
		if (nodeConfig.ChangeStatus & ChangeBootDiskSize) > 0 {
			for _, cmd := range compute["ssh_node_bootdisk_resize_commands"].([]interface{}) {
				var cmdOutput string
				log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					return
				}
				if len(cmdOutput) > 0 && log.IsLevelEnabled(log.DebugLevel) {
					log.Debugf("Completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
		if (nodeConfig.ChangeStatus & ChangeDataDiskSize) > 0 {
			for _, cmd := range compute["ssh_node_datadisk_resize_commands"].([]interface{}) {
				var cmdOutput string
				log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					return
				}
				if len(cmdOutput) > 0 && log.IsLevelEnabled(log.DebugLevel) {
					log.Debugf("Completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
	}
	return
}

func resourceUpdateServerSnapshot(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var oldSnapState, newSnapState, snapStateInter, snapStorage []string
	var sshPrivateKey string
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldSnapshot, newSnapshot := d.GetChange("snapshot")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(snapshot): failure: %s", err)
		return
	}
	for _, snapshot := range oldSnapshot.([]interface{}) {
		oldSnapState = append(oldSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	for _, snapshot := range newSnapshot.([]interface{}) {
		newSnapState = append(newSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	snapStateInter = stringSliceIntersection(oldSnapState, newSnapState)
	if snapStorage, err = ontap.GetSnapshots(nodeConfig); err != nil {
		err = fmt.Errorf("resourceUpdateServer(snapshot): %s", err)
		return
	}
	for _, name := range oldSnapState {
		if stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
			if err = ontap.DeleteSnapshot(nodeConfig, name); err != nil {
				err = fmt.Errorf("resourceUpdateServer(snapshot): %s", err)
				return
			}
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSnapshotDelete
		}
	}
	for _, name := range newSnapState {
		if !stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
			for _, snapshot := range newSnapshot.([]interface{}) {
				if snapshot.(map[string]interface{})["name"].(string) == name {
					if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
						if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
							err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
						} else {
							err = fmt.Errorf("resourceUpdateServer(snapshot): expected compute.ssh_user and compute.ssh_private_key parameters to ensure fsfreeze for snapshot %s", name)
						}
					} else {
						err = ontap.CreateSnapshot(nodeConfig, name, "")
					}
					if err != nil {
						err = fmt.Errorf("resourceUpdateServer(snapshot): %s", err)
						return
					}
					nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSnapshotCreate
				}
			}
		}
	}
	return
}

func resourceUpdateServerMaintenance(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, nodeState string
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	_, newMaintenance := d.GetChange("maintenance")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if len(newMaintenance.([]interface{})) == 0 {
		return
	}
	maintenance := newMaintenance.([]interface{})[0].(map[string]interface{})
	if !maintenance["execute"].(bool) || len(maintenance["tasks"].([]interface{})) == 0 {
		return
	}
	log.Infof("Running Server Maintenance Tasks")
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil || powerState == "down" {
		return
	}
	var rancherNode rancher.RancherNode
	if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, false); err != nil {
	        err = fmt.Errorf("resourceUpdateServer(maintenance): error: %s", err)
		meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
	        return
	}
	if maintenance["synchronized_run"].(bool) {
	        err = meta.(*config.FlexbotConfig).UpdateManagerAcquire()
	        defer meta.(*config.FlexbotConfig).UpdateManagerRelease()
	        if err != nil {
		        err = fmt.Errorf("resourceUpdateServer(maintenance): last resource instance update returned error: %s", err)
		        return
		}
	}
	for _, task := range maintenance["tasks"].([]interface{}) {
                switch task.(string) {
                case "cordon":
                        if rancherNode.IsNodeWorker() {
	                        if err = rancherNode.RancherAPINodeCordon(); err != nil {
	                                err = fmt.Errorf("resourceUpdateServer(maintenance): cordon error: %s", err)
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        return
		                }
		        }
		        nodeState = "cordoned"
                case "uncordon":
                        if rancherNode.IsNodeWorker() {
			        if err = rancherNode.RancherAPINodeUncordon(); err != nil {
				        err = fmt.Errorf("resourceUpdateServer(maintenance): uncordon error: %s", err)
				        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				        return
			        }
			}
		        nodeState = "active"
                case "drain":
                        if rancherNode.IsNodeWorker() {
	                        if err = rancherNode.RancherAPINodeCordonDrain(); err != nil {
	                                err = fmt.Errorf("resourceUpdateServer(maintenance): drain error: %s", err)
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        return
		                }
		        }
		        nodeState = "cordoned,drained"
                case "restart":
	                var sshPrivateKey string
                        sshUser := compute["ssh_user"].(string)
                        if len(compute["ssh_private_key"].(string)) > 0 {
                                if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
                                        err = fmt.Errorf("resourceUpdateServer(maintenance): decryptAttribute() failure: %s", err)
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
                                        return
                                }
                        }
			if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
                                if err = shutdownServer(nodeConfig, sshUser, sshPrivateKey); err != nil {
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        return
                                }
	                        if err = ucsm.StartServer(nodeConfig); err != nil {
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        return
	                        }
		                if err = waitForSSH(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
		                        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			                return
		                }
			} else {
	                        if err = ucsm.StopServer(nodeConfig); err == nil {
	                                time.Sleep(NodeGraceShutdownTimeout * time.Second)
                                        err = ucsm.StartServer(nodeConfig)
	                        }
	                        if err != nil {
			                err = fmt.Errorf("resourceUpdateServer(maintenance): restart error: %s", err)
			                meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			                return
                                }
			}
		        nodeState = "cordoned,drained,active"
                }
	}
	if maintenance["wait_for_node_timeout"].(int) > 0 {
	        if err = rancherNode.RancherAPINodeWaitForState(nodeState, maintenance["wait_for_node_timeout"].(int)); err != nil {
		        err = fmt.Errorf("resourceUpdateServer(maintenance): rancherAPINodeWaitForState() error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	}
	if maintenance["node_grace_timeout"].(int) > 0 {
	        if err = rancherNode.RancherAPINodeWaitForGracePeriod(maintenance["node_grace_timeout"].(int)); err != nil {
		        err = fmt.Errorf("resourceUpdateServer(maintenance): rancherAPINodeWaitForGracePeriod() error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	}
        return
}

func resourceUpdateServerRestore(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, sshPrivateKey string
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldRestore, newRestore := d.GetChange("restore")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(restore): failure: %s", err)
		return
	}
	if len(newRestore.([]interface{})) == 0 {
		return
	}
	restore := newRestore.([]interface{})[0].(map[string]interface{})
	if !restore["restore"].(bool) {
		return
	}
	log.Infof("Restoring Server Storage from snapshot")
	if restore["snapshot_name"] == nil || len(restore["snapshot_name"].(string)) == 0 {
		var lastSnapshot string
		var snapshotList []string
		tRFC3339exp, _ := regexp.Compile(`20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]-[0-9][0-9]:[0-9][0-9]$`)
		lastSnapshotCreated := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		if snapshotList, err = ontap.GetSnapshots(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
			return
		}
		for _, snapshot := range snapshotList {
			timeFormatted := tRFC3339exp.FindString(snapshot)
			if len(timeFormatted) > 0 {
				if snapshotCreated, err := time.Parse(time.RFC3339, timeFormatted); err == nil {
					if snapshotCreated.Unix() > lastSnapshotCreated.Unix() {
						lastSnapshotCreated = snapshotCreated
						lastSnapshot = snapshot
					}
				}
			}
			if len(lastSnapshot) > 0 {
				restore["snapshot_name"] = lastSnapshot
			}
		}
	}
	if restore["snapshot_name"] == nil || len(restore["snapshot_name"].(string)) == 0 {
		err = fmt.Errorf("resourceUpdateServer(restore): snapshot not found, expected snapshot_name")
		return
	}
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		err = fmt.Errorf("resourceUpdateServer(restore): server %s has power state up", nodeConfig.Compute.HostName)
		return
	}
	if powerState == "up" {
		if err = ucsm.StopServer(nodeConfig); err != nil {
			return
		}
	}
	if err = ontap.RestoreSnapshot(nodeConfig, restore["snapshot_name"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
		return
	}
	if err = ontap.LunRestoreMapping(nodeConfig); err != nil {
		err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
		return
	}
	if err = ucsm.StartServer(nodeConfig); err != nil {
		return
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
		if err = waitForSSH(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
			return
		}
	}
	nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSnapshotRestore
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.Set("restore", oldRestore)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	return
}

func resourceUpdateServerLabels(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var rancherNode rancher.RancherNode
        meta.(*config.FlexbotConfig).Sync.Lock()
	oldLabels, newLabels := d.GetChange("labels")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true); err == nil {
		err = rancherNode.RancherAPINodeUpdateLabels(oldLabels.(map[string]interface{}), newLabels.(map[string]interface{}))
	}
	return
}

func resourceUpdateServerTaints(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var rancherNode rancher.RancherNode
        meta.(*config.FlexbotConfig).Sync.Lock()
	oldTaints, newTaints := d.GetChange("taints")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true); err == nil {
		err = rancherNode.RancherAPINodeUpdateTaints(oldTaints.([]interface{}), newTaints.([]interface{}))
	}
	return
}

func resourceDeleteServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var powerState string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Deleting Server %s", nodeConfig.Compute.HostName)
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): server %s has power state up", nodeConfig.Compute.HostName))
		return
	}
	var rancherNode rancher.RancherNode
	if rancherNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, false); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
		return
	}
	// Cordon/drain worker node
	if powerState == "up" && rancherNode.IsNodeWorker() {
	        if err = rancherNode.RancherAPINodeCordonDrain(); err != nil {
		        diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
			return
		}
        }
	// Delete node
	if err = rancherNode.RancherAPINodeDelete(); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
		return
	}
	// Wait for etcd/controlplane node cluster update
	if powerState == "up" && (rancherNode.IsNodeEtcd() || rancherNode.IsNodeControlPlane()) {
	        rancherNode.RancherAPIClusterWaitForTransitioning(Wait4ClusterTransitioningTimeout)
		if err = rancherNode.RancherAPIClusterWaitForState("active", rancher.Wait4ClusterStateTimeout); err != nil {
		        diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
			return
		}
        }
	if powerState == "up" {
		if err = ucsm.StopServer(nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
        }
	if err = ucsm.DeleteServer(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ucsm.DeleteServer()",
			Detail:   err.Error(),
		})
	}
	if err = ontap.DeleteBootStorage(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.DeleteBootStorage()",
			Detail:   err.Error(),
		})
	}
	var ipamProvider ipam.IpamProvider
	if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ipam.NewProvider()",
			Detail:   err.Error(),
		})
		return
	}
	if err = ipamProvider.Release(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ipamProvider.Release()",
			Detail:   err.Error(),
		})
	}
	return
}

func resourceImportServer(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if diags := resourceReadServer(ctx, d, meta); diags != nil && len(diags) > 0 {
		return nil, fmt.Errorf("%s: %s", diags[0].Summary, diags[0].Detail)
	}
	return schema.ImportStatePassthroughContext(ctx, d, meta)
}

func setFlexbotInput(d *schema.ResourceData, meta interface{}) (nodeConfig *config.NodeConfig, err error) {
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	var exists bool
	if nodeConfig, exists = meta.(*config.FlexbotConfig).NodeConfig[compute["hostname"].(string)]; exists {
		return
	}
	nodeConfig = &config.NodeConfig{}
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	pIpam := p.Get("ipam").([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.Provider = pIpam["provider"].(string)
	nodeConfig.Ipam.DnsZone = pIpam["dns_zone"].(string)
	ibCredentials := pIpam["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.IbCredentials.Host = ibCredentials["host"].(string)
	nodeConfig.Ipam.IbCredentials.User = ibCredentials["user"].(string)
	nodeConfig.Ipam.IbCredentials.Password = ibCredentials["password"].(string)
	nodeConfig.Ipam.IbCredentials.WapiVersion = ibCredentials["wapi_version"].(string)
	nodeConfig.Ipam.IbCredentials.DnsView = ibCredentials["dns_view"].(string)
	nodeConfig.Ipam.IbCredentials.NetworkView = ibCredentials["network_view"].(string)
	pCompute := p.Get("compute").([]interface{})[0].(map[string]interface{})
	ucsmCredentials := pCompute["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.UcsmCredentials.Host = ucsmCredentials["host"].(string)
	nodeConfig.Compute.UcsmCredentials.User = ucsmCredentials["user"].(string)
	nodeConfig.Compute.UcsmCredentials.Password = ucsmCredentials["password"].(string)
	pStorage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := pStorage["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.CdotCredentials.Host = cdotCredentials["host"].(string)
	nodeConfig.Storage.CdotCredentials.User = cdotCredentials["user"].(string)
	nodeConfig.Storage.CdotCredentials.Password = cdotCredentials["password"].(string)
	nodeConfig.Storage.CdotCredentials.ApiMethod = cdotCredentials["api_method"].(string)
	nodeConfig.Storage.CdotCredentials.ZapiVersion = cdotCredentials["zapi_version"].(string)
	nodeConfig.Compute.SpOrg = compute["sp_org"].(string)
	nodeConfig.Compute.SpTemplate = compute["sp_template"].(string)
	nodeConfig.Compute.Description = compute["description"].(string)
	nodeConfig.Compute.Label = compute["label"].(string)
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Compute.BladeSpec.Dn = bladeSpec["dn"].(string)
		nodeConfig.Compute.BladeSpec.Model = bladeSpec["model"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCpus = bladeSpec["num_of_cpus"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCores = bladeSpec["num_of_cores"].(string)
		nodeConfig.Compute.BladeSpec.NumOfThreads = bladeSpec["num_of_threads"].(string)
		nodeConfig.Compute.BladeSpec.TotalMemory = bladeSpec["total_memory"].(string)
	}
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SvmName = storage["svm_name"].(string)
	nodeConfig.Storage.ImageRepoName = storage["image_repo_name"].(string)
	nodeConfig.Storage.VolumeName = storage["volume_name"].(string)
	nodeConfig.Storage.IgroupName = storage["igroup_name"].(string)
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.BootLun.Name = bootLun["name"].(string)
	nodeConfig.Storage.BootLun.Size = bootLun["size"].(int)
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SeedLun.Name = seedLun["name"].(string)
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Storage.DataLun.Name = dataLun["name"].(string)
		nodeConfig.Storage.DataLun.Size = dataLun["size"].(int)
	}
	if len(storage["data_nvme"].([]interface{})) > 0 {
		dataNvme := storage["data_nvme"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Storage.DataNvme.Namespace = dataNvme["namespace"].(string)
		nodeConfig.Storage.DataNvme.Subsystem = dataNvme["subsystem"].(string)
		nodeConfig.Storage.DataNvme.Size = dataNvme["size"].(int)
	}
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	for i := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.Node = append(nodeConfig.Network.Node, config.NetworkInterface{})
		nodeConfig.Network.Node[i].Name = node["name"].(string)
		nodeConfig.Network.Node[i].Macaddr = node["macaddr"].(string)
		nodeConfig.Network.Node[i].Ip = node["ip"].(string)
		nodeConfig.Network.Node[i].Fqdn = node["fqdn"].(string)
		nodeConfig.Network.Node[i].Subnet = node["subnet"].(string)
		nodeConfig.Network.Node[i].IpRange = node["ip_range"].(string)
		nodeConfig.Network.Node[i].Gateway = node["gateway"].(string)
		nodeConfig.Network.Node[i].DnsServer1 = node["dns_server1"].(string)
		nodeConfig.Network.Node[i].DnsServer2 = node["dns_server2"].(string)
		nodeConfig.Network.Node[i].DnsServer3 = node["dns_server3"].(string)
		nodeConfig.Network.Node[i].DnsDomain = node["dns_domain"].(string)
		nodeConfig.Network.Node[i].Parameters = make(map[string]string)
		for paramKey, paramValue := range node["parameters"].(map[string]interface{}) {
			nodeConfig.Network.Node[i].Parameters[paramKey] = paramValue.(string)
		}
	}
	for i := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.IscsiInitiator = append(nodeConfig.Network.IscsiInitiator, config.IscsiInitiator{})
		nodeConfig.Network.IscsiInitiator[i].Name = initiator["name"].(string)
		nodeConfig.Network.IscsiInitiator[i].Ip = initiator["ip"].(string)
		nodeConfig.Network.IscsiInitiator[i].Fqdn = initiator["fqdn"].(string)
		nodeConfig.Network.IscsiInitiator[i].Subnet = initiator["subnet"].(string)
		nodeConfig.Network.IscsiInitiator[i].IpRange = initiator["ip_range"].(string)
		nodeConfig.Network.IscsiInitiator[i].Gateway = initiator["gateway"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer1 = initiator["dns_server1"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer2 = initiator["dns_server2"].(string)
		nodeConfig.Network.IscsiInitiator[i].Parameters = make(map[string]string)
		for paramKey, paramValue := range initiator["parameters"].(map[string]interface{}) {
			nodeConfig.Network.IscsiInitiator[i].Parameters[paramKey] = paramValue.(string)
		}
		nodeConfig.Network.IscsiInitiator[i].InitiatorName = initiator["initiator_name"].(string)
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		if len(initiator["iscsi_target"].([]interface{})) > 0 {
			nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = initiator["iscsi_target"].([]interface{})[0].(map[string]interface{})["node_name"].(string)
			for _, targetAddr := range initiator["iscsi_target"].([]interface{})[0].(map[string]interface{})["interfaces"].([]interface{}) {
				nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, targetAddr.(string))
			}
		}
	}
	for i := range network["nvme_host"].([]interface{}) {
		nvmeHost := network["nvme_host"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.NvmeHost = append(nodeConfig.Network.NvmeHost, config.NvmeHost{})
		nodeConfig.Network.NvmeHost[i].HostInterface = nvmeHost["host_interface"].(string)
		nodeConfig.Network.NvmeHost[i].Ip = nvmeHost["ip"].(string)
		nodeConfig.Network.NvmeHost[i].Subnet = nvmeHost["subnet"].(string)
		nodeConfig.Network.NvmeHost[i].HostNqn = nvmeHost["host_nqn"].(string)
		nodeConfig.Network.NvmeHost[i].NvmeTarget = &config.NvmeTarget{}
		if len(nvmeHost["nvme_target"].([]interface{})) > 0 {
			nodeConfig.Network.NvmeHost[i].NvmeTarget.TargetNqn = nvmeHost["nvme_target"].([]interface{})[0].(map[string]interface{})["target_nqn"].(string)
			for _, targetAddr := range nvmeHost["nvme_target"].([]interface{})[0].(map[string]interface{})["interfaces"].([]interface{}) {
				nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces = append(nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces, targetAddr.(string))
			}
		}
	}
	nodeConfig.CloudArgs = make(map[string]string)
	for argKey, argValue := range d.Get("cloud_args").(map[string]interface{}) {
		nodeConfig.CloudArgs[argKey] = argValue.(string)
	}
	nodeConfig.Labels = make(map[string]string)
	for labelKey, labelValue := range d.Get("labels").(map[string]interface{}) {
		nodeConfig.Labels[labelKey] = labelValue.(string)
	}
	nodeConfig.Taints = make([]rancherManagementClient.Taint, 0)
	for _, taint := range d.Get("taints").([]interface{}) {
		nodeConfig.Taints = append(
                        nodeConfig.Taints,
                        rancherManagementClient.Taint{
                                Key: taint.(map[string]interface{})["key"].(string),
                                Value: taint.(map[string]interface{})["value"].(string),
                                Effect: taint.(map[string]interface{})["effect"].(string),
                        })
	}
	if err = config.SetDefaults(nodeConfig, compute["hostname"].(string), bootLun["os_image"].(string), seedLun["seed_template"].(string), p.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	} else {
		meta.(*config.FlexbotConfig).NodeConfig[compute["hostname"].(string)] = nodeConfig
	}
	return
}

func setFlexbotOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) {
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	compute["sp_dn"] = nodeConfig.Compute.SpDn
	bladeAssigned := make(map[string]interface{})
	bladeAssigned["dn"] = nodeConfig.Compute.BladeAssigned.Dn
	bladeAssigned["model"] = nodeConfig.Compute.BladeAssigned.Model
	bladeAssigned["serial"] = nodeConfig.Compute.BladeAssigned.Serial
	bladeAssigned["num_of_cpus"] = nodeConfig.Compute.BladeAssigned.NumOfCpus
	bladeAssigned["num_of_cores"] = nodeConfig.Compute.BladeAssigned.NumOfCores
	bladeAssigned["num_of_threads"] = nodeConfig.Compute.BladeAssigned.NumOfThreads
	bladeAssigned["total_memory"] = nodeConfig.Compute.BladeAssigned.TotalMemory
	if len(compute["blade_assigned"].([]interface{})) > 0 {
		compute["blade_assigned"].([]interface{})[0] = bladeAssigned
	} else {
		compute["blade_assigned"] = append(compute["blade_assigned"].([]interface{}), bladeAssigned)
	}
	compute["powerstate"] = nodeConfig.Compute.Powerstate
	compute["description"] = nodeConfig.Compute.Description
	compute["label"] = nodeConfig.Compute.Label
	storage["svm_name"] = nodeConfig.Storage.SvmName
	storage["image_repo_name"] = nodeConfig.Storage.ImageRepoName
	storage["volume_name"] = nodeConfig.Storage.VolumeName
	storage["igroup_name"] = nodeConfig.Storage.IgroupName
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	bootLun["name"] = nodeConfig.Storage.BootLun.Name
	bootLun["id"] = nodeConfig.Storage.BootLun.Id
	if nodeConfig.Storage.BootLun.Size > 0 {
		bootLun["size"] = nodeConfig.Storage.BootLun.Size
	}
	if nodeConfig.Storage.BootLun.OsImage.Name != "" {
		bootLun["os_image"] = nodeConfig.Storage.BootLun.OsImage.Name
	}
	storage["boot_lun"].([]interface{})[0] = bootLun
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	seedLun["name"] = nodeConfig.Storage.SeedLun.Name
	seedLun["id"] = nodeConfig.Storage.SeedLun.Id
	if nodeConfig.Storage.SeedLun.SeedTemplate.Location != "" {
		seedLun["seed_template"] = nodeConfig.Storage.SeedLun.SeedTemplate.Location
	}
	storage["seed_lun"].([]interface{})[0] = seedLun
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		dataLun["name"] = nodeConfig.Storage.DataLun.Name
		dataLun["id"] = nodeConfig.Storage.DataLun.Id
		if nodeConfig.Storage.DataLun.Size > 0 {
			dataLun["size"] = nodeConfig.Storage.DataLun.Size
		}
		storage["data_lun"].([]interface{})[0] = dataLun
	}
	if len(storage["data_nvme"].([]interface{})) > 0 {
		dataNvme := storage["data_nvme"].([]interface{})[0].(map[string]interface{})
		dataNvme["namespace"] = nodeConfig.Storage.DataNvme.Namespace
		dataNvme["subsystem"] = nodeConfig.Storage.DataNvme.Subsystem
		if nodeConfig.Storage.DataNvme.Size > 0 {
			dataNvme["size"] = nodeConfig.Storage.DataNvme.Size
		}
		storage["data_nvme"].([]interface{})[0] = dataNvme
	}
	storage["snapshots"] = []string{}
	for _, snapshot := range nodeConfig.Storage.Snapshots {
		storage["snapshots"] = append(storage["snapshots"].([]string), snapshot)
	}
	storage["force_update"] = false
	for i := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		node["macaddr"] = nodeConfig.Network.Node[i].Macaddr
		node["ip"] = nodeConfig.Network.Node[i].Ip
		node["fqdn"] = nodeConfig.Network.Node[i].Fqdn
		network["node"].([]interface{})[i] = node
	}
	for i := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		initiator["ip"] = nodeConfig.Network.IscsiInitiator[i].Ip
		initiator["initiator_name"] = nodeConfig.Network.IscsiInitiator[i].InitiatorName
		initiator["fqdn"] = nodeConfig.Network.IscsiInitiator[i].Fqdn
		initiator["subnet"] = nodeConfig.Network.IscsiInitiator[i].Subnet
		initiator["gateway"] = nodeConfig.Network.IscsiInitiator[i].Gateway
		initiator["dns_server1"] = nodeConfig.Network.IscsiInitiator[i].DnsServer1
		initiator["dns_server2"] = nodeConfig.Network.IscsiInitiator[i].DnsServer2
		if len(initiator["iscsi_target"].([]interface{})) == 0 {
			if nodeConfig.Network.IscsiInitiator[i].IscsiTarget != nil {
				iscsiTarget := make(map[string]interface{})
				iscsiTarget["node_name"] = nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName
				iscsiTarget["interfaces"] = []string{}
				for _, iface := range nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces {
					iscsiTarget["interfaces"] = append(iscsiTarget["interfaces"].([]string), iface)
				}
				initiator["iscsi_target"] = append(initiator["iscsi_target"].([]interface{}), iscsiTarget)
			}
		}
		network["iscsi_initiator"].([]interface{})[i] = initiator
	}
	for i := range network["nvme_host"].([]interface{}) {
		nvmeHost := network["nvme_host"].([]interface{})[i].(map[string]interface{})
		nvmeHost["host_interface"] = nodeConfig.Network.NvmeHost[i].HostInterface
		nvmeHost["ip"] = nodeConfig.Network.NvmeHost[i].Ip
		nvmeHost["subnet"] = nodeConfig.Network.NvmeHost[i].Subnet
		nvmeHost["host_nqn"] = nodeConfig.Network.NvmeHost[i].HostNqn
		if len(nvmeHost["nvme_target"].([]interface{})) == 0 {
			if nodeConfig.Network.NvmeHost[i].NvmeTarget != nil {
				nvmeTarget := make(map[string]interface{})
				nvmeTarget["target_nqn"] = nodeConfig.Network.NvmeHost[i].NvmeTarget.TargetNqn
				nvmeTarget["interfaces"] = []string{}
				for _, iface := range nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces {
					nvmeTarget["interfaces"] = append(nvmeTarget["interfaces"].([]string), iface)
				}
				nvmeHost["nvme_target"] = append(nvmeHost["nvme_target"].([]interface{}), nvmeTarget)
			}
		}
		network["nvme_host"].([]interface{})[i] = nvmeHost
	}
	labels := make(map[string]interface{})
	for labelKey, labelValue := range nodeConfig.Labels {
	        labels[labelKey] = labelValue
	}
	taints := make([]interface{}, 0)
	for _, taint := range nodeConfig.Taints {
	        taints = append(taints, taint)
	}
	d.Set("compute", []interface{}{compute})
	d.Set("network", []interface{}{network})
	d.Set("storage", []interface{}{storage})
	d.Set("labels", labels)
	d.Set("taints", taints)
}

func decryptAttribute(meta interface{}, encrypted string) (decrypted string, err error) {
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	if decrypted, err = crypt.DecryptString(encrypted, meta.(*config.FlexbotConfig).FlexbotProvider.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("decryptAttribute(): failure to decrypt: %s", err)
	}
	return
}

func createSnapshot(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string, snapshotName string) (err error) {
	var filesystems, freezeCmds, unfreezeCmds, errs []string
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var bStdout, bStderr bytes.Buffer
	var exists bool
	if exists, err = ontap.SnapshotExists(nodeConfig, snapshotName); exists || err != nil {
		return
	}
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", nodeConfig.Network.Node[0].Ip+":22", config); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to connect to host %s: %s", nodeConfig.Network.Node[0].Ip, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	sess.Stdout = &bStdout
	sess.Stderr = &bStderr
	err = sess.Run(`cat /proc/mounts | sed -n 's/^\/dev\/mapper\/[^ ]\+[ ]\+\(\/[^ \/]\{1,64\}\).*/\1/p' | uniq`)
	sess.Close()
	if err != nil {
		err = fmt.Errorf("createSnapshot(): failed to run command: %s: %s", err, bStderr.String())
		return
	}
	if bStdout.Len() > 0 {
		filesystems = strings.Split(strings.Trim(bStdout.String(), "\n"), "\n")
	}
	unfreezeCmds = append(unfreezeCmds, "fsfreeze -u /")
	for _, fs := range filesystems {
		freezeCmds = append(freezeCmds, "fsfreeze -f "+fs)
		unfreezeCmds = append(unfreezeCmds, "fsfreeze -u "+fs)
	}
	freezeCmds = append(freezeCmds, "fsfreeze -f /")
	cmd := fmt.Sprintf(`sudo -n sh -c 'sync && sleep 5 && sync && %s && (echo -n frozen && sleep 5); %s'`, strings.Join(freezeCmds, " && "), strings.Join(unfreezeCmds, "; "))
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	bStdout.Reset()
	bStderr.Reset()
	sess.Stdout = &bStdout
	sess.Stderr = &bStderr
	if err = sess.Start(cmd); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to start SSH command: %s", err)
		return
	}
	for i := 0; i < 30; i++ {
		if bStdout.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if bStdout.String() == "frozen" {
		if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, "fsfreeze did not complete, snapshot is not created")
	}
	if err = sess.Wait(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to run SSH command: %s: %s", err, bStderr.String()))
	}
	if err != nil {
		err = fmt.Errorf("createSnapshot(): %s", strings.Join(errs, " , "))
	}
	return
}

func waitForSSH(nodeConfig *config.NodeConfig, waitForSSHTimeout int, sshUser string, sshPrivateKey string) (err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(waitForSSHTimeout))
	restartTime := time.Now().Add(time.Second * NodeRestartTimeout)
	for time.Now().Before(giveupTime) {
		if checkSSHListen(nodeConfig.Network.Node[0].Ip) {
			if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				stabilazeTime := time.Now().Add(time.Second * 60)
				for time.Now().Before(stabilazeTime) {
					if err = checkSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey); err == nil {
						break
					}
					time.Sleep(5 * time.Second)
				}
			}
			if err == nil {
				break
			}
		}
		time.Sleep(5 * time.Second)
		if time.Now().After(restartTime) {
			ucsm.StopServer(nodeConfig)
			ucsm.StartServer(nodeConfig)
			restartTime = time.Now().Add(time.Second * NodeRestartTimeout)
		}
	}
	if time.Now().After(giveupTime) && err != nil {
		err = fmt.Errorf("waitForSsh(): exceeded timeout %d: %s", waitForSSHTimeout, err)
	}
	return
}

func shutdownServer(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string) (err error) {
        var powerState string
        // Trying graceful node shutdown
	if _, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, "sudo shutdown -h 0"); err == nil {
	        waitForShutdown := time.Now().Add(time.Second * time.Duration(NodeGraceShutdownTimeout))
	        for time.Now().Before(waitForShutdown) {
	                if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			        return
		        }
		        if powerState == "down" {
		                break
		        }
		        time.Sleep(5 * time.Second)
	        }
	}
	err = ucsm.StopServer(nodeConfig)
        return
}
