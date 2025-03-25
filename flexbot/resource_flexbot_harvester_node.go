package flexbot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ipam"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ucsm"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/rancher"
)

const (
	ServerBootTimeout = 300
	ServerPowerStateTimeout = 60
	HarvesterInstallerStage1Timeout = 1800
	HarvesterInstallerStage2Timeout = 1800
	CheckNodeReadyTimeout           = 5
)

func resourceFlexbotHarvesterNode() *schema.Resource {
	return &schema.Resource{
		Schema:        schemaHarvesterNode(),
		CreateContext: resourceCreateHarvesterNode,
		ReadContext:   resourceReadHarvesterNode,
		UpdateContext: resourceUpdateHarvesterNode,
		DeleteContext: resourceDeleteHarvesterNode,
		Importer: &schema.ResourceImporter{
			StateContext: resourceImportHarvesterNode,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(7200 * time.Second),
			Update: schema.DefaultTimeout(28800 * time.Second),
			Delete: schema.DefaultTimeout(1800 * time.Second),
		},
	}
}

func resourceCreateHarvesterNode(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var errs []error
	var nodeConfig *config.NodeConfig
	var sshPrivateKey string
	if nodeConfig, err = setFlexbotHarvesterNodeInput(d, meta); err != nil {
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
	log.Infof("Creating Harvester Node %s", nodeConfig.Compute.HostName)
	var nodeExists bool
	if nodeExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if nodeExists {
		diags = diag.FromErr(fmt.Errorf("resourceCreateHarvesterNode(): node %s already exists", nodeConfig.Compute.HostName))
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
	if err = ontap.CreateHarvesterStoragePreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.CreateHarvesterStoragePreflight()",
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
		diags = diag.FromErr(fmt.Errorf("resourceCreateHarvesterNode(): %s", err))
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.SetId(nodeConfig.Compute.HostName)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	for i := 0; i < StorageRetryAttempts; i++ {
		if err = ontap.CreateHarvesterStorage(nodeConfig); err == nil {
			break
		}
		errs = append(errs, err)
		time.Sleep(StorageRetryTimeout * time.Second)
		ontap.DeleteHarvesterStorage(nodeConfig)
		time.Sleep(time.Duration(StorageRetryTimeout * (i + 1)) * time.Second)
	}
	if err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		for i := 0; i < StorageRetryAttempts; i++ {
			if err = ontap.CreateSeedStorage(nodeConfig); err == nil {
				break
			}
			errs = append(errs, err)
			time.Sleep(time.Duration(StorageRetryTimeout * (i + 1)) * time.Second)
		}
	}
	if err == nil {
		if err = ucsm.StartServer(nodeConfig); err == nil {
			err = waitForHostNetwork(nodeConfig, ServerBootTimeout)
		}
	} else {
		ontap.DeleteHarvesterStorage(nodeConfig)
	}
	if err == nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
		d.SetConnInfo(map[string]string{"type": "ssh", "host": nodeConfig.Network.Node[0].Ip})
		meta.(*config.FlexbotConfig).Sync.Unlock()
		if err = waitForOperationalState(nodeConfig, "power-off", HarvesterInstallerStage1Timeout); err == nil {
			ucsm.StopServer(nodeConfig)
			if err = waitForPowerState(nodeConfig, "down", ServerPowerStateTimeout); err == nil {
				if err = ontap.RemapHarvesterStorage(nodeConfig); err == nil {
					err = ucsm.StartServer(nodeConfig)
					if err == nil && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
						waitForSshTimeout := HarvesterInstallerStage2Timeout
						if compute["wait_for_ssh_timeout"].(int) > 0 {
							waitForSshTimeout = compute["wait_for_ssh_timeout"].(int)
						}
						if err = waitForSSH(nodeConfig, waitForSshTimeout, sshUser, sshPrivateKey); err == nil {
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
				}
			}
		}
	}
	setFlexbotHarvesterNodeOutput(d, meta, nodeConfig)
	if err == nil {
		_, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true)
	}
	if err != nil {
		errs = append(errs, err)
		for _, err = range errs {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "resourceCreateHarvesterNode()",
				Detail:   err.Error(),
			})
		}
	}
	return
}

func resourceReadHarvesterNode(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotHarvesterNodeInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Refreshing Harvester Node %s", nodeConfig.Compute.HostName)
	var nodeExists bool
	if nodeExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	var storageExists bool
	if storageExists, err = ontap.DiscoverHarvesterStorage(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if nodeExists && storageExists {
		var ipamProvider ipam.IpamProvider
		if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
			diags = diag.FromErr(err)
			return
		}
		if err = ipamProvider.Discover(nodeConfig); err != nil {
			diags = diag.FromErr(err)
			return
		}
		setFlexbotHarvesterNodeOutput(d, meta, nodeConfig)
	} else {
                meta.(*config.FlexbotConfig).Sync.Lock()
		d.SetId("")
                meta.(*config.FlexbotConfig).Sync.Unlock()
	}
	return
}

func resourceUpdateHarvesterNode(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	var isNew, isCompute, isStorage bool
	if nodeConfig, err = setFlexbotHarvesterNodeInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	isNew = d.IsNewResource()
        isCompute = d.HasChange("compute")
        isStorage = d.HasChange("storage")
        if isCompute || isStorage {
	        d.Partial(true)
        }
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if isCompute && !isNew {
		if err = resourceUpdateHarvesterNodeCompute(d, meta, nodeConfig); err != nil {
			resourceReadHarvesterNode(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if isStorage && !isNew {
		if err = resourceUpdateHarvesterNodeStorage(d, meta, nodeConfig); err != nil {
			resourceReadHarvesterNode(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	resourceReadHarvesterNode(ctx, d, meta)
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.Partial(false)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	return

	return
}

func resourceUpdateHarvesterNodeCompute(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, newPowerState, operState, sshPrivateKey string
	var oldBladeSpec, newBladeSpec map[string]interface{}
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldCompute, newCompute := d.GetChange("compute")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	sshUser := compute["ssh_user"].(string)
	if sshPrivateKey, err = decryptAttribute(meta, compute["ssh_private_key"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateHarvesterNode(compute): failure: %s", err)
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
					err = fmt.Errorf("resourceUpdateHarvesterNode(compute):  regexp.MatchString(%s), error: %s", newBladeSpec[specItem].(string), err)
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
					err = fmt.Errorf("resourceUpdateHarvesterNode(compute): unexpected value %s=%s, error: %s", specItem, compute["blade_assigned"].([]interface{})[0].(map[string]interface{})[specItem].(string), err)
					return
				}
				if inRange, err = valueInRange(newBladeSpec[specItem].(string), specValue); err != nil {
					err = fmt.Errorf("resourceUpdateHarvesterNode(compute): unexpected blade_spec value %s=%s, error: %s", specItem, newBladeSpec[specItem].(string), err)
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
	if operState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
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
			err = fmt.Errorf("resourceUpdateHarvesterNode(compute): last resource instance update returned error: %s", err)
			return
		}
		log.Infof("Updating Harvester node %s", nodeConfig.Compute.HostName)
	        if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
	        	log.Infof("Running compute  preflight check")
	                if err = ucsm.UpdateServerPreflight(nodeConfig); err != nil {
			        err = fmt.Errorf("resourceUpdateHarvesterNode(compute): error: %s", err)
			        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			        return
	                }
	        }
		if powerState == "up" {
			if operState == "ok" {
				var harvesterNode rancher.RancherNode
				if harvesterNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true); err != nil {
					err = fmt.Errorf("resourceUpdateHarvesterNode(compute): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				if err = harvesterNode.RancherAPINodeEnableMaintainanceMode(meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
					err = fmt.Errorf("resourceUpdateHarvesterNode(compute): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
					log.Infof("Shutting down node %s", nodeConfig.Compute.HostName)
                                	if err = shutdownServer(nodeConfig, sshUser, sshPrivateKey); err != nil {
		                        	meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		                        	return
                                	}
				}
			}
			log.Infof("Power off node %s", nodeConfig.Compute.HostName)
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
			log.Infof("Changing blade specification for node %s", nodeConfig.Compute.HostName)
			if err = ucsm.UpdateServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if newPowerState == "up" {
			log.Infof("Power on node %s", nodeConfig.Compute.HostName)
			if err = ucsm.StartServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			waitForSshTimeout := HarvesterInstallerStage2Timeout
			if compute["wait_for_ssh_timeout"].(int) > 0 {
				waitForSshTimeout = compute["wait_for_ssh_timeout"].(int)
			}
			if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				log.Infof("Waiting for node %s on network", nodeConfig.Compute.HostName)
				if err = waitForSSH(nodeConfig, waitForSshTimeout, sshUser, sshPrivateKey); err != nil {
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			var harvesterNode rancher.RancherNode
			if harvesterNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, true); err != nil {
				err = fmt.Errorf("resourceUpdateHarvesterNode(compute): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if err = harvesterNode.RancherAPINodeDisableMaintainanceMode(meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
				err = fmt.Errorf("resourceUpdateHarvesterNode(compute): error: %s", err)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
	}
	if (oldCompute.([]interface{})[0].(map[string]interface{}))["description"].(string) != (newCompute.([]interface{})[0].(map[string]interface{}))["description"].(string) ||
		(oldCompute.([]interface{})[0].(map[string]interface{}))["label"].(string) != (newCompute.([]interface{})[0].(map[string]interface{}))["label"].(string) {
		err = ucsm.UpdateServerAttributes(nodeConfig)
	}
	return
}

func resourceUpdateHarvesterNodeStorage(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	return
}

func resourceDeleteHarvesterNode(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var powerState, operState string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotHarvesterNodeInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Deleting Harvester Node %s", nodeConfig.Compute.HostName)
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if operState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if powerState == "up" && operState == "ok" && compute["safe_removal"].(bool) {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteHarvesterNode(): node %s has power state up", nodeConfig.Compute.HostName))
		return
	}
	var harvesterNode rancher.RancherNode
	if harvesterNode, err = rancher.RancherAPIInitialize(d, meta, nodeConfig, false); err != nil {
		err = fmt.Errorf("resourceDeleteHarvesterNode(): error: %s", err)
		diags = diag.FromErr(err)
		return
	}
	if powerState == "up" && operState == "ok" {
		if err = harvesterNode.RancherAPINodeGetID(d, meta); err == nil {
			if err = harvesterNode.RancherAPINodeWaitUntilReady(CheckNodeReadyTimeout); err == nil {
				if err = harvesterNode.RancherAPINodeEnableMaintainanceMode(meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
					err = fmt.Errorf("resourceDeleteHarvesterNode(): error: %s", err)
					diags = diag.FromErr(err)
					return
				}
			}
		}
	}
	if err = harvesterNode.RancherAPINodeDelete(); err != nil {
		err = fmt.Errorf("resourceDeleteHarvesterNode(): error: %s", err)
		diags = diag.FromErr(err)
		return
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
	if err = ontap.DeleteHarvesterStorage(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.DeleteHarvesterStorage()",
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

func resourceImportHarvesterNode(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if diags := resourceReadHarvesterNode(ctx, d, meta); diags != nil && len(diags) > 0 {
		return nil, fmt.Errorf("%s: %s", diags[0].Summary, diags[0].Detail)
	}
	return schema.ImportStatePassthroughContext(ctx, d, meta)
}

func setFlexbotHarvesterNodeInput(d *schema.ResourceData, meta interface{}) (nodeConfig *config.NodeConfig, err error) {
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
	nodeConfig.Ipam.IbCredentials.ExtAttributes = make(map[string]interface{})
	for attrName, attrValue := range ibCredentials["ext_attributes"].(map[string]interface{}) {
		nodeConfig.Ipam.IbCredentials.ExtAttributes[attrName] = attrValue
	}
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
	nodeConfig.Storage.BootLun.Id = bootLun["id"].(int)
	nodeConfig.Storage.BootLun.Size = bootLun["size"].(int)
	bootstrapLun := storage["bootstrap_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.BootstrapLun.Name = bootstrapLun["name"].(string)
	nodeConfig.Storage.BootstrapLun.Id = bootstrapLun["id"].(int)
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SeedLun.Name = seedLun["name"].(string)
	nodeConfig.Storage.SeedLun.Id = seedLun["id"].(int)
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
		nodeConfig.Network.IscsiInitiator[i].Macaddr = initiator["macaddr"].(string)
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
	nodeConfig.CloudArgs = make(map[string]string)
	for argKey, argValue := range d.Get("cloud_args").(map[string]interface{}) {
		nodeConfig.CloudArgs[argKey] = argValue.(string)
	}
	if err = config.SetDefaults(nodeConfig, compute["hostname"].(string), bootstrapLun["os_image"].(string), seedLun["seed_template"].(string), p.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	} else {
		meta.(*config.FlexbotConfig).NodeConfig[compute["hostname"].(string)] = nodeConfig
	}
	return
}

func setFlexbotHarvesterNodeOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) {
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
	storage["boot_lun"].([]interface{})[0] = bootLun
	bootstrapLun := storage["bootstrap_lun"].([]interface{})[0].(map[string]interface{})
	bootstrapLun["name"] = nodeConfig.Storage.BootstrapLun.Name
	bootstrapLun["id"] = nodeConfig.Storage.BootstrapLun.Id
	if nodeConfig.Storage.BootstrapLun.OsImage.Name != "" {
		bootstrapLun["os_image"] = nodeConfig.Storage.BootstrapLun.OsImage.Name
	}
	storage["bootstrap_lun"].([]interface{})[0] = bootstrapLun
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	seedLun["name"] = nodeConfig.Storage.SeedLun.Name
	seedLun["id"] = nodeConfig.Storage.SeedLun.Id
	if nodeConfig.Storage.SeedLun.SeedTemplate.Location != "" {
		seedLun["seed_template"] = nodeConfig.Storage.SeedLun.SeedTemplate.Location
	}
	storage["seed_lun"].([]interface{})[0] = seedLun
	for i := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		node["macaddr"] = nodeConfig.Network.Node[i].Macaddr
		node["ip"] = nodeConfig.Network.Node[i].Ip
		node["fqdn"] = nodeConfig.Network.Node[i].Fqdn
		network["node"].([]interface{})[i] = node
	}
	for i := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		initiator["macaddr"] = nodeConfig.Network.IscsiInitiator[i].Macaddr
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
	d.Set("compute", []interface{}{compute})
	d.Set("network", []interface{}{network})
	d.Set("storage", []interface{}{storage})
}
