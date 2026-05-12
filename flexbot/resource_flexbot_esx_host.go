package flexbot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ipam"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ucsm"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/vmware"
)

const (
	EsxBootTimeout           = 600
	EsxShutdownTimeout       = 180
	EsxInstallerTimeout      = 1800
	EsxEnterMaintModeTimeout = 900
	EsxExitMaintModeTimeout =  60
	VMwareApiTimeout         = 15
)

func resourceFlexbotEsxHost() *schema.Resource {
	return &schema.Resource{
		Schema:        schemaEsxHost(),
		CreateContext: resourceCreateEsxHost,
		ReadContext:   resourceReadEsxHost,
		UpdateContext: resourceUpdateEsxHost,
		DeleteContext: resourceDeleteEsxHost,
		Importer: &schema.ResourceImporter{
			StateContext: resourceImportEsxHost,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(7200 * time.Second),
			Update: schema.DefaultTimeout(28800 * time.Second),
			Delete: schema.DefaultTimeout(1800 * time.Second),
		},
	}
}

func resourceCreateEsxHost(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var errs []error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotEsxHostInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Creating ESX Node %s", nodeConfig.Compute.HostName)
	var nodeExists bool
	if nodeExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if nodeExists {
		diags = diag.FromErr(fmt.Errorf("resourceCreateEsxHost(): node %s already exists", nodeConfig.Compute.HostName))
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
	if err = ontap.CreateEsxStoragePreflight(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.CreateEsxStoragePreflight()",
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
	if len(diags) > 0 {
		return
	}
	if err = ipamProvider.Allocate(nodeConfig); err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceCreateEsxHost(): %s", err))
		return
	}
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.SetId(nodeConfig.Compute.HostName)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if err = ontap.CreateEsxStorage(nodeConfig); err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		err = ucsm.StartServer(nodeConfig)
	} else {
		ontap.DeleteEsxStorage(nodeConfig)
		ucsm.DeleteServer(nodeConfig)
		ipamProvider.Release(nodeConfig)
	}
	if err == nil {
		setFlexbotEsxHostOutput(d, meta, nodeConfig)
		var waitForHostInstallerTimeout int
		if meta.(*config.FlexbotConfig).VMwareConfig.WaitForHostInstallerTimeout > 0 {
			waitForHostInstallerTimeout = meta.(*config.FlexbotConfig).VMwareConfig.WaitForHostInstallerTimeout
		} else {
			waitForHostInstallerTimeout = EsxInstallerTimeout
		}
		_, err = waitForEsxHost(d, meta, nodeConfig, waitForHostInstallerTimeout)
	}
	if err != nil {
		errs = append(errs, err)
		for _, err = range errs {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "resourceCreateEsxHost()",
				Detail:   err.Error(),
			})
		}
	}
	if err != nil {
		errs = append(errs, err)
		for _, err = range errs {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "resourceCreateEsxHost()",
				Detail:   err.Error(),
			})
		}
	}
	return
}

func resourceReadEsxHost(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotEsxHostInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Refreshing ESX Node %s", nodeConfig.Compute.HostName)
	var nodeExists bool
	if nodeExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		diags = diag.FromErr(err)
		return
	}
	var storageExists bool
	if storageExists, err = ontap.DiscoverEsxStorage(nodeConfig); err != nil {
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
		setFlexbotEsxHostOutput(d, meta, nodeConfig)
	} else {
                meta.(*config.FlexbotConfig).Sync.Lock()
		d.SetId("")
                meta.(*config.FlexbotConfig).Sync.Unlock()
	}
	return
}

func resourceUpdateEsxHost(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	var isNew, isCompute, isStorage bool
	if nodeConfig, err = setFlexbotEsxHostInput(d, meta); err != nil {
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
		if err = resourceUpdateEsxHostCompute(d, meta, nodeConfig); err != nil {
			resourceReadEsxHost(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	if isStorage && !isNew {
		if err = resourceUpdateEsxHostStorage(d, meta, nodeConfig); err != nil {
			resourceReadEsxHost(ctx, d, meta)
			diags = diag.FromErr(err)
			return
		}
	}
	resourceReadEsxHost(ctx, d, meta)
        meta.(*config.FlexbotConfig).Sync.Lock()
	d.Partial(false)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	return
}

func resourceUpdateEsxHostCompute(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, newPowerState, operState string
	var oldBladeSpec, newBladeSpec map[string]interface{}
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldCompute, newCompute := d.GetChange("compute")
        meta.(*config.FlexbotConfig).Sync.Unlock()
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
					err = fmt.Errorf("resourceUpdateEsxHost(compute):  regexp.MatchString(%s), error: %s", newBladeSpec[specItem].(string), err)
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
					err = fmt.Errorf("resourceUpdateEsxHost(compute): unexpected value %s=%s, error: %s", specItem, compute["blade_assigned"].([]interface{})[0].(map[string]interface{})[specItem].(string), err)
					return
				}
				if inRange, err = valueInRange(newBladeSpec[specItem].(string), specValue); err != nil {
					err = fmt.Errorf("resourceUpdateEsxHost(compute): unexpected blade_spec value %s=%s, error: %s", specItem, newBladeSpec[specItem].(string), err)
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
			err = fmt.Errorf("resourceUpdateEsxHost(compute): last resource instance update returned error: %s", err)
			return
		}
		log.Infof("Updating ESX node %s", nodeConfig.Compute.HostName)
	        if (nodeConfig.ChangeStatus & ChangeBladeSpec) > 0 {
	        	log.Infof("Running compute  preflight check")
	                if err = ucsm.UpdateServerPreflight(nodeConfig); err != nil {
			        err = fmt.Errorf("resourceUpdateEsxHost(compute): error: %s", err)
			        meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			        return
	                }
	        }
		if powerState == "up" {
			if operState == "ok" {
				log.Infof("Shutting down ESX host %s", nodeConfig.Compute.HostName)
				if err = shutdownEsxHost(d, meta, nodeConfig); err != nil {
					err = fmt.Errorf("resourceUpdateEsxHost(compute): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
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
			log.Infof("Power on ESX host %s", nodeConfig.Compute.HostName)
			if err = ucsm.StartServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			waitForHostBootTimeout := meta.(*config.FlexbotConfig).VMwareConfig.WaitForHostBootTimeout
			if waitForHostBootTimeout == 0 {
				waitForHostBootTimeout = EsxBootTimeout
			}
			var vmwareAPI vmware.VMwareAPI
			if vmwareAPI, err = waitForEsxHost(d, meta, nodeConfig, waitForHostBootTimeout); err == nil && vmwareAPI != nil {
				if err = vmwareAPI.VMwareAPIExitMaintenanceMode(EsxExitMaintModeTimeout); err != nil {
					err = fmt.Errorf("resourceUpdateEsxHost(compute): error: %s", err)
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

func resourceUpdateEsxHostStorage(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var powerState, operState string
        meta.(*config.FlexbotConfig).Sync.Lock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	oldStorage, newStorage := d.GetChange("storage")
        meta.(*config.FlexbotConfig).Sync.Unlock()
	oldBootLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	newBootLun := (newStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	if oldBootLun["installer_image"].(string) != newBootLun["installer_image"].(string) || oldBootLun["size"].(int) != newBootLun["size"].(int) || oldBootLun["kickstart_template"].(string) != newBootLun["kickstart_template"].(string) {
		if oldBootLun["installer_image"].(string) != newBootLun["installer_image"].(string) || oldBootLun["size"].(int) != newBootLun["size"].(int) {
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeOsImage
		}
		if oldBootLun["kickstart_template"].(string) != newBootLun["kickstart_template"].(string) {
			nodeConfig.ChangeStatus = nodeConfig.ChangeStatus | ChangeSeedTemplate
		}
		log.Infof("Updating Storage image for ESX host %s", nodeConfig.Compute.HostName)
		err = meta.(*config.FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*config.FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateEsxHost(storage): last resource instance update returned error: %s", err)
			return
		}
                nodeConfig.Storage.BootLun.OsImage.Name = filepath.Base(newBootLun["installer_image"].(string))
                nodeConfig.Storage.BootLun.OsImage.Location = newBootLun["installer_image"].(string)
                nodeConfig.Storage.SeedLun.SeedTemplate.Name = filepath.Base(newBootLun["kickstart_template"].(string))
                nodeConfig.Storage.SeedLun.SeedTemplate.Location = newBootLun["kickstart_template"].(string)
		log.Infof("Running ESX host storage preflight check")
		if err = ontap.CreateEsxStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateEsxHost(storage): ESX host storage preflight check error: %s", err)
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if operState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" && operState == "ok" {
			if compute["safe_removal"].(bool) {
				err = fmt.Errorf("resourceUpdateEsxHost(storage): ESX host %s has power state up", nodeConfig.Compute.HostName)
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if powerState == "up" {
			if operState == "ok" {
				log.Infof("Shutting down ESX host %s", nodeConfig.Compute.HostName)
				if err = shutdownEsxHost(d, meta, nodeConfig); err != nil {
					err = fmt.Errorf("resourceUpdateEsxHost(storage): error: %s", err)
					meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			log.Infof("Power off ESX host %s", nodeConfig.Compute.HostName)
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		log.Infof("Re-provision Storage for ESX host %s", nodeConfig.Compute.HostName)
		for i := 0; i < StorageRetryAttempts; i++ {
			if err = ontap.DeleteEsxStorage(nodeConfig); err == nil {
				if err = ontap.CreateEsxStorage(nodeConfig); err == nil {
					break
				}
				time.Sleep(StorageRetryTimeout * time.Second)
				ontap.DeleteEsxStorage(nodeConfig)
				time.Sleep(time.Duration(StorageRetryTimeout * (i + 1)) * time.Second)
			}
		}
		if err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		log.Infof("Power on ESX host %s", nodeConfig.Compute.HostName)
		if err = ucsm.StartServer(nodeConfig); err == nil {
			err = waitForHostNetwork(nodeConfig, EsxInstallerTimeout)
		} else {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		var waitForHostInstallerTimeout int
		if meta.(*config.FlexbotConfig).VMwareConfig.WaitForHostInstallerTimeout > 0 {
			waitForHostInstallerTimeout = meta.(*config.FlexbotConfig).VMwareConfig.WaitForHostInstallerTimeout
		} else {
			waitForHostInstallerTimeout = EsxInstallerTimeout
		}
		_, err = waitForEsxHost(d, meta, nodeConfig, waitForHostInstallerTimeout)
		if err != nil {
			meta.(*config.FlexbotConfig).UpdateManagerSetError(err)
		}
	}
	return
}

func resourceDeleteEsxHost(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var powerState, operState string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotEsxHostInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	log.Infof("Deleting ESX Node %s", nodeConfig.Compute.HostName)
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
		diags = diag.FromErr(fmt.Errorf("resourceDeleteEsxHost(): node %s has power state up", nodeConfig.Compute.HostName))
		return
	}
	if powerState == "up" {
		if operState == "ok" {
			log.Infof("Shutting down ESX host %s", nodeConfig.Compute.HostName)
			if err = shutdownEsxHost(d, meta, nodeConfig); err != nil {
				err = fmt.Errorf("resourceDeleteEsxHost(): error: %s", err)
				diags = diag.FromErr(err)
				return
			}
		}
		log.Infof("Power off ESX host %s", nodeConfig.Compute.HostName)
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
	if err = ontap.DeleteEsxStorage(nodeConfig); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "ontap.DeleteEsxStorage()",
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

func resourceImportEsxHost(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if diags := resourceReadEsxHost(ctx, d, meta); diags != nil && len(diags) > 0 {
		return nil, fmt.Errorf("%s: %s", diags[0].Summary, diags[0].Detail)
	}
	return schema.ImportStatePassthroughContext(ctx, d, meta)
}

func setFlexbotEsxHostInput(d *schema.ResourceData, meta interface{}) (nodeConfig *config.NodeConfig, err error) {
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
	nodeConfig.Compute.Firmware = compute["firmware"].(string)
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Compute.BladeSpec.Dn = bladeSpec["dn"].(string)
		nodeConfig.Compute.BladeSpec.Model = bladeSpec["model"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCpus = bladeSpec["num_of_cpus"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCores = bladeSpec["num_of_cores"].(string)
		nodeConfig.Compute.BladeSpec.NumOfThreads = bladeSpec["num_of_threads"].(string)
		nodeConfig.Compute.BladeSpec.TotalMemory = bladeSpec["total_memory"].(string)
	}
	if len(compute["chassis_id"].(string)) > 0 {
		nodeConfig.Compute.ChassisId = compute["chassis_id"].(string)
	}
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SvmName = storage["svm_name"].(string)
	nodeConfig.Storage.VolumeName = storage["volume_name"].(string)
	nodeConfig.Storage.IgroupName = storage["igroup_name"].(string)
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.BootLun.Name = bootLun["name"].(string)
	nodeConfig.Storage.BootLun.Id = bootLun["id"].(int)
	nodeConfig.Storage.BootLun.Size = bootLun["size"].(int)
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
	if err = config.SetDefaults(nodeConfig, compute["hostname"].(string), bootLun["installer_image"].(string), bootLun["kickstart_template"].(string), p.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	} else {
		meta.(*config.FlexbotConfig).NodeConfig[compute["hostname"].(string)] = nodeConfig
	}
	return
}

func setFlexbotEsxHostOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) {
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
	compute["chassis_id"] = nodeConfig.Compute.ChassisId
	compute["powerstate"] = nodeConfig.Compute.Powerstate
	compute["description"] = nodeConfig.Compute.Description
	compute["label"] = nodeConfig.Compute.Label
	compute["firmware"] = nodeConfig.Compute.Firmware
	storage["svm_name"] = nodeConfig.Storage.SvmName
	storage["volume_name"] = nodeConfig.Storage.VolumeName
	storage["igroup_name"] = nodeConfig.Storage.IgroupName
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	bootLun["name"] = nodeConfig.Storage.BootLun.Name
	bootLun["id"] = nodeConfig.Storage.BootLun.Id
	if nodeConfig.Storage.BootLun.Size > 0 {
		bootLun["size"] = nodeConfig.Storage.BootLun.Size
	}
	storage["boot_lun"].([]interface{})[0] = bootLun
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

func shutdownEsxHost(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
        var operState string
	var vmwareAPI vmware.VMwareAPI
	if vmwareAPI, err = vmware.VMwareAPIInitialize(d, meta, nodeConfig); err == nil && vmwareAPI != nil {
		if err = vmwareAPI.VMwareAPIEnterMaintenanceMode(EsxEnterMaintModeTimeout); err == nil {
			if err = vmwareAPI.VMwareAPIShutdownHost(EsxShutdownTimeout); err == nil {
				giveupTime := time.Now().Add(time.Second * time.Duration(EsxShutdownTimeout))
				for time.Now().Before(giveupTime) {
					if operState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
						return
					}
					if operState == "power-off" {
						break
					}
					time.Sleep(5 * time.Second)
				}
				if time.Now().After(giveupTime) {
					if  err == nil {
						err = fmt.Errorf("exceeded timeout %d", EsxShutdownTimeout)
					} else {
						err = fmt.Errorf("exceeded timeout %d: %s", EsxShutdownTimeout, err)
					}
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("shutdownEsxHost(%s): %s", nodeConfig.Compute.HostName, err)
	}
        return
}

func waitForEsxHost(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForHostTimeout int) (vmwareAPI vmware.VMwareAPI, err error) {
	var hostState string
	if vmwareAPI, err = vmware.VMwareAPIInitialize(d, meta, nodeConfig); vmwareAPI == nil && err == nil {
		return
	}
	giveupTime := time.Now().Add(time.Second * time.Duration(waitForHostTimeout))
	for time.Now().Before(giveupTime) {
		if vmwareAPI, err = vmware.VMwareAPIInitialize(d, meta, nodeConfig); err == nil && vmwareAPI != nil {
			if hostState, err = vmwareAPI.VMwareAPIGetHostState(VMwareApiTimeout); err == nil && hostState == "connected" {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}
	if time.Now().After(giveupTime) {
		if  err == nil {
			err = fmt.Errorf("waitForEsxHost(ip=%s): exceeded timeout %d, host state=%s", nodeConfig.Network.Node[0].Ip, waitForHostTimeout, hostState)
		} else {
			err = fmt.Errorf("waitForEsxHost(ip=%s): exceeded timeout %d: %s", nodeConfig.Network.Node[0].Ip, waitForHostTimeout, err)
		}
	}
	return
}
