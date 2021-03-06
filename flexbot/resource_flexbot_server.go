package flexbot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"bytes"
	"time"
	"context"
	"golang.org/x/crypto/ssh"

	"github.com/denisbrodbeck/machineid"
        "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
        "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
        log "github.com/sirupsen/logrus"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ipam"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ucsm"
)

const (
        NODE_RESTART_TIMEOUT = 600
)

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
		Schema: schemaFlexbotServer(),
		CreateContext: resourceCreateServer,
		ReadContext:   resourceReadServer,
		UpdateContext: resourceUpdateServer,
		DeleteContext: resourceDeleteServer,
		Importer: &schema.ResourceImporter{
			StateContext: resourceImportServer,
		},
                Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(3600 * time.Second),
			Update: schema.DefaultTimeout(7200 * time.Second),
			Delete: schema.DefaultTimeout(1800 * time.Second),
                },
	}
}

func resourceCreateServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
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
	d.SetId(nodeConfig.Compute.HostName)
	err = ontap.CreateBootStorage(nodeConfig)
	if err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		err = ontap.CreateSeedStorage(nodeConfig)
	}
	if err == nil {
		err = ucsm.StartServer(nodeConfig)
	}
	if err == nil {
		d.SetConnInfo(map[string]string{"type": "ssh", "host": nodeConfig.Network.Node[0].Ip,})
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 && err == nil {
		if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
			for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
				var cmdOutput string
				log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
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
		var rancherNode *RancherNode
		if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, true); err == nil {
			err = rancherNode.rancherApiNodeSetAnnotationsLabels()
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
		setFlexbotOutput(d, meta, nodeConfig)
	} else {
		d.SetId("")
	}
	return
}

func resourceUpdateServer(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if d.HasChange("compute") && !d.IsNewResource() {
		if err = resourceUpdateServerCompute(d, meta, nodeConfig); err != nil {
			resourceReadServer(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if d.HasChange("storage") && !d.IsNewResource() {
		if err = resourceUpdateServerStorage(d, meta, nodeConfig); err != nil {
			resourceReadServer(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if d.HasChange("snapshot") && !d.IsNewResource() {
		if err = resourceUpdateServerSnapshot(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if d.HasChange("labels") {
		if err = resourceUpdateServerLabels(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	if d.HasChange("restore") {
		if err = resourceUpdateServerRestore(d, meta, nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	resourceReadServer(ctx, d, meta)
	d.Partial(false)
	if (nodeConfig.ChangeStatus & (ChangeBladeSpec | ChangeOsImage | ChangeSeedTemplate | ChangeSnapshotRestore)) > 0 {
		var rancherNode *RancherNode
		if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, true); err == nil {
			if err = rancherNode.rancherApiNodeSetAnnotationsLabels(); err != nil {
				diags = diag.FromErr(err)
			}
		} else {
			diags = diag.FromErr(err)
		}
	}
	return
}

func resourceUpdateServerCompute(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, newPowerState string
	var oldBladeSpec, newBladeSpec map[string]interface{}
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	oldCompute, newCompute := d.GetChange("compute")
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
				if matched == false {
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
				if inRange == false {
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
	newPowerState = (newCompute.([]interface{})[0].(map[string]interface{}))["powerstate"].(string)
	if  newPowerState != powerState {
		nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangePowerState
	}
	if (nodeConfig.ChangeStatus & (ChangeBladeSpec | ChangePowerState)) > 0 {
		err = meta.(*FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateServer(compute): last resource instance update returned error: %s", err)
			return
		}
		log.Infof("Updating Server Compute for node %s", nodeConfig.Compute.HostName)
		var rancherNode *RancherNode
		if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, false); err != nil {
			err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" {
			if (newCompute.([]interface{})[0].(map[string]interface{}))["safe_removal"].(bool) {
				err = fmt.Errorf("resourceUpdateServer(compute): server %s has power state up", nodeConfig.Compute.HostName)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			// Cordon/drain worker nodes
			if rancherNode.NodeWorker {
				if err = rancherNode.rancherApiNodeCordon(); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				// Trying graceful node shutdown
				if _, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, "sudo shutdown -h 0"); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): runSshCommand(shutdown) error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				waitForShutdown := time.Now().Add(time.Second * time.Duration(60))
				for time.Now().Before(waitForShutdown) {
					if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
						meta.(*FlexbotConfig).UpdateManagerSetError(err)
						return
					}
					if powerState == "down" {
						break
					}
					time.Sleep(1 * time.Second)
				}
			}
			if powerState == "up" {
				if err = ucsm.StopServer(nodeConfig); err != nil {
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
		}
		if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
			if err = ucsm.UpdateServer(nodeConfig); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if newPowerState == "up" {
			if err = ucsm.StartServer(nodeConfig); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				if err = waitForSsh(nodeConfig, (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			// Uncordon worker nodes
			if rancherNode.NodeWorker {
				if err = rancherNode.rancherApiNodeUncordon(); err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if err = rancherNode.rancherApiClusterWaitForState("active", WAIT4CLUSTER_STATE_TIMEOUT); err != nil {
				err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if meta.(*FlexbotConfig).NodeGraceTimeout > 0 {
				time.Sleep(time.Duration(meta.(*FlexbotConfig).NodeGraceTimeout) * time.Second)
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
	var powerState string
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	oldStorage, newStorage := d.GetChange("storage")
	oldBootLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	newBootLun := (newStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	oldSeedLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	newSeedLun := (newStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	if oldBootLun["os_image"].(string) != newBootLun["os_image"].(string) || oldSeedLun["seed_template"].(string) != newSeedLun["seed_template"].(string) {
		if oldBootLun["os_image"].(string) != newBootLun["os_image"].(string) {
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeOsImage
		}
		if oldSeedLun["seed_template"].(string) != newSeedLun["seed_template"].(string) {
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSeedTemplate
		}
		log.Infof("Updating Server Storage image for node %s", nodeConfig.Compute.HostName)
		err = meta.(*FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): last resource instance update returned error: %s", err)
			return
		}
		if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		nodeConfig.Storage.BootLun.OsImage.Name = newBootLun["os_image"].(string)
		nodeConfig.Storage.SeedLun.SeedTemplate.Location = newSeedLun["seed_template"].(string)
		log.Infof("Running boot storage preflight check")
		if err = ontap.CreateBootStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): boot storage preflight check error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		log.Infof("Running seed storage preflight check")
		if err = ontap.CreateSeedStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): seed storage preflight check error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" && compute["safe_removal"].(bool) {
			err = fmt.Errorf("resourceUpdateServer(storage): server %s has power state up", nodeConfig.Compute.HostName)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		var rancherNode *RancherNode
		if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, false); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" {
			// Cordon/drain worker nodes
			if rancherNode.NodeWorker {
				if err = rancherNode.rancherApiNodeCordon(); err != nil {
					err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		// Delete etcd/controlplane node
		if rancherNode.NodeEtcd || rancherNode.NodeControlPlane {
			if err = rancherNode.rancherApiNodeDelete(); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if (newStorage.([]interface{})[0].(map[string]interface{}))["auto_snapshot_on_update"].(bool) {
			t := time.Now()
			snapshotName := fmt.Sprintf("terraform:%s:%s-%s", oldBootLun["os_image"].(string), oldSeedLun["seed_template"].(string), t.Format(time.RFC3339))
			if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = ontap.DeleteBootLUNs(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateBootStorage(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateSeedStorage(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ucsm.StartServer(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if compute["wait_for_ssh_timeout"].(int) > 0  && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
			if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
				if err = rancherNode.rancherApiClusterWaitForState("active", WAIT4CLUSTER_STATE_TIMEOUT); err != nil {
					err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				if err = rancherNode.rancherApiClusterWaitForState("active", WAIT4CLUSTER_STATE_TIMEOUT); err != nil {
					err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
					var cmdOutput string
					log.Infof("Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
					if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
						meta.(*FlexbotConfig).UpdateManagerSetError(err)
						return
					}
					if len(cmdOutput) > 0 && log.IsLevelEnabled(log.DebugLevel) {
						log.Debugf("Completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
					}
				}
			}
		}
		if rancherNode.NodeEtcd || rancherNode.NodeControlPlane {
			rancherNode.rancherApiClusterWaitForState("updating", 60)
		}
		// Uncordon worker nodes
		if rancherNode.NodeWorker {
			if err = rancherNode.rancherApiNodeUncordon(); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = rancherNode.rancherApiClusterWaitForState("active", WAIT4CLUSTER_STATE_TIMEOUT); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if meta.(*FlexbotConfig).NodeGraceTimeout > 0 {
			time.Sleep(time.Duration(meta.(*FlexbotConfig).NodeGraceTimeout) * time.Second)
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
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
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
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
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
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	oldSnapshot, newSnapshot := d.GetChange("snapshot")
	for _, snapshot := range oldSnapshot.([]interface{}) {
		oldSnapState = append(oldSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	for _, snapshot := range newSnapshot.([]interface{}) {
		newSnapState = append(newSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	snapStateInter = stringSliceIntersection(oldSnapState, newSnapState)
	if snapStorage, err = ontap.GetSnapshots(nodeConfig); err != nil {
		err = fmt.Errorf("resourceUpdateServer(): %s", err)
		return
	}
	for _, name := range oldSnapState {
		if stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
			if err = ontap.DeleteSnapshot(nodeConfig, name); err != nil {
				err = fmt.Errorf("resourceUpdateServer(): %s", err)
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
							err = fmt.Errorf("expected compute.ssh_user and compute.ssh_private_key parameters to ensure fsfreeze for snapshot %s", name)
						}
					} else {
						err = ontap.CreateSnapshot(nodeConfig, name, "")
					}
					if err != nil {
						err = fmt.Errorf("resourceUpdateServer(): %s", err)
						return
					}
					nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSnapshotCreate
				}
			}
		}
	}
	return
}

func resourceUpdateServerRestore(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState string
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	oldRestore, newRestore := d.GetChange("restore")
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
		if snapshotList, err =  ontap.GetSnapshots(nodeConfig); err != nil {
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
	if compute["wait_for_ssh_timeout"].(int) > 0  && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
		if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
			return
		}
	}
	nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSnapshotRestore
	d.Set("restore", oldRestore)
	return
}

func resourceUpdateServerLabels(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var rancherNode *RancherNode
	oldLabels, newLabels := d.GetChange("labels")
	if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, true); err == nil {
		err = rancherNode.rancherApiNodeUpdateLabels(oldLabels.(map[string]interface{}), newLabels.(map[string]interface{}))
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
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): server %s has power state up", nodeConfig.Compute.HostName))
		return
	}
	var rancherNode *RancherNode
	if rancherNode, err = rancherApiInitialize(d, meta, nodeConfig, false); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
		return
	}
	if powerState == "up" {
		// Cordon/drain worker nodes
		if rancherNode.NodeWorker {
			if err = rancherNode.rancherApiNodeCordon(); err != nil {
				diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
				return
			}
		}
		if err = ucsm.StopServer(nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
	}
	// Delete node
	if err = rancherNode.rancherApiNodeDelete(); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteServer(): error: %s", err))
		return
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
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	var exists bool
	if nodeConfig, exists = meta.(*FlexbotConfig).NodeConfig[compute["hostname"].(string)]; exists {
		return
	}
	nodeConfig = &config.NodeConfig{}
	p := meta.(*FlexbotConfig).FlexbotProvider
	p_ipam := p.Get("ipam").([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.Provider = p_ipam["provider"].(string)
	nodeConfig.Ipam.DnsZone = p_ipam["dns_zone"].(string)
	ibCredentials := p_ipam["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.IbCredentials.Host = ibCredentials["host"].(string)
	nodeConfig.Ipam.IbCredentials.User = ibCredentials["user"].(string)
	nodeConfig.Ipam.IbCredentials.Password = ibCredentials["password"].(string)
	nodeConfig.Ipam.IbCredentials.WapiVersion = ibCredentials["wapi_version"].(string)
	nodeConfig.Ipam.IbCredentials.DnsView = ibCredentials["dns_view"].(string)
	nodeConfig.Ipam.IbCredentials.NetworkView = ibCredentials["network_view"].(string)
	p_compute := p.Get("compute").([]interface{})[0].(map[string]interface{})
	ucsmCredentials := p_compute["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.UcsmCredentials.Host = ucsmCredentials["host"].(string)
	nodeConfig.Compute.UcsmCredentials.User = ucsmCredentials["user"].(string)
	nodeConfig.Compute.UcsmCredentials.Password = ucsmCredentials["password"].(string)
	p_storage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := p_storage["credentials"].([]interface{})[0].(map[string]interface{})
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
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	for i, _ := range network["node"].([]interface{}) {
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
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
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
			for _, target_addr := range initiator["iscsi_target"].([]interface{})[0].(map[string]interface{})["interfaces"].([]interface{}) {
                    		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, target_addr.(string))
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
	passPhrase := p.Get("pass_phrase").(string)
	if passPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			return nil, err
		}
	}
	if err = config.SetDefaults(nodeConfig, compute["hostname"].(string), bootLun["os_image"].(string), seedLun["seed_template"].(string), passPhrase); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	} else {
		meta.(*FlexbotConfig).NodeConfig[compute["hostname"].(string)] = nodeConfig
	}
	return
}

func setFlexbotOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) {
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
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
	storage["snapshots"] = []string{}
	for _, snapshot := range nodeConfig.Storage.Snapshots {
		storage["snapshots"] = append(storage["snapshots"].([]string), snapshot)
	}
	for i, _ := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		node["macaddr"] = nodeConfig.Network.Node[i].Macaddr
		node["ip"] = nodeConfig.Network.Node[i].Ip
		node["fqdn"] = nodeConfig.Network.Node[i].Fqdn
		network["node"].([]interface{})[i] = node
	}
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
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
				iscsi_target := make(map[string]interface{})
				iscsi_target["node_name"] = nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName
				iscsi_target["interfaces"] = []string{}
				for _, iface := range nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces {
					iscsi_target["interfaces"] = append(iscsi_target["interfaces"].([]string), iface)
				}
				initiator["iscsi_target"] = append(initiator["iscsi_target"].([]interface{}), iscsi_target)
			}
		}
		network["iscsi_initiator"].([]interface{})[i] = initiator
	}
	d.Set("compute", []interface{}{compute})
	d.Set("network", []interface{}{network})
	d.Set("storage", []interface{}{storage})
}

func createSnapshot(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string, snapshotName string) (err error) {
	var filesystems, freeze_cmds, unfreeze_cmds, errs []string
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var b_stdout, b_stderr bytes.Buffer
	var exists bool
	if exists, err = ontap.SnapshotExists(nodeConfig, snapshotName); exists || err != nil {
		return
	}
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", nodeConfig.Network.Node[0].Ip + ":22", config); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to connect to host %s: %s", nodeConfig.Network.Node[0].Ip, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	err = sess.Run(`cat /proc/mounts | sed -n 's/^\/dev\/mapper\/[^ ]\+[ ]\+\(\/[^ \/]\{1,64\}\).*/\1/p' | uniq`)
	sess.Close()
	if err != nil {
		err = fmt.Errorf("createSnapshot(): failed to run command: %s: %s", err, b_stderr.String())
		return
	}
	if b_stdout.Len() > 0 {
		filesystems = strings.Split(strings.Trim(b_stdout.String(), "\n"), "\n")
	}
	unfreeze_cmds = append(unfreeze_cmds, "fsfreeze -u /")
	for _, fs := range filesystems {
		freeze_cmds = append(freeze_cmds, "fsfreeze -f " + fs)
		unfreeze_cmds = append(unfreeze_cmds, "fsfreeze -u " + fs)
	}
	freeze_cmds = append(freeze_cmds, "fsfreeze -f /")
	cmd := fmt.Sprintf(`sudo -n sh -c 'sync && sleep 5 && sync && %s && (echo -n frozen && sleep 5); %s'`, strings.Join(freeze_cmds, " && "), strings.Join(unfreeze_cmds, "; "))
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	b_stdout.Reset()
	b_stderr.Reset()
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	if err = sess.Start(cmd); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to start SSH command: %s", err)
		return
	}
	for i := 0; i < 30; i++ {
		if b_stdout.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if b_stdout.String() == "frozen" {
		if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, "fsfreeze did not complete, snapshot is not created")
	}
	if err = sess.Wait(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to run SSH command: %s: %s", err, b_stderr.String()))
	}
	if err != nil {
		err = fmt.Errorf("createSnapshot(): %s", strings.Join(errs, " , "))
	}
	return
}

func waitForSsh(nodeConfig *config.NodeConfig, waitForSshTimeout int, sshUser string, sshPrivateKey string) (err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(waitForSshTimeout))
	restartTime := time.Now().Add(time.Second * NODE_RESTART_TIMEOUT)
	for time.Now().Before(giveupTime) {
		if checkSshListen(nodeConfig.Network.Node[0].Ip) {
			if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				stabilazeTime := time.Now().Add(time.Second * 60)
				for time.Now().Before(stabilazeTime) {
					if err = checkSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey); err == nil {
						break
					}
					time.Sleep(1 * time.Second)
				}
			}
			if err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
		if time.Now().After(restartTime) {
			ucsm.StopServer(nodeConfig)
			ucsm.StartServer(nodeConfig)
			restartTime = time.Now().Add(time.Second * NODE_RESTART_TIMEOUT)
		}
	}
	if time.Now().After(giveupTime) && err != nil {
		err = fmt.Errorf("waitForSsh(): exceeded timeout %d: %s", waitForSshTimeout, err)
	}
	return
}
