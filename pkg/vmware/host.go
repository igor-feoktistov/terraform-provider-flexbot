package vmware

import (
	"time"
	"context"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	ApiInitTimeout = 5
)

// VsphereEsxHost is vSphere ESXi host definition
type EsxHost struct {
        HostClient *govmomi.Client
        HostSystem *object.HostSystem
	NodeConfig *config.NodeConfig
}

func EsxHostAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (host *EsxHost, err error) {
        var hostClient *govmomi.Client
        var hostSystem *object.HostSystem
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ApiInitTimeout) * time.Second)
	defer cancel()
        vmwareConfig := meta.(*config.FlexbotConfig).VMwareConfig
	if hostClient, hostSystem, err = initializeHostClient(ctx, nodeConfig.Network.Node[0].Ip, vmwareConfig.HostUsername, vmwareConfig.HostPassword); err == nil {
		host = &EsxHost{
			HostClient: hostClient,
			HostSystem: hostSystem,
			NodeConfig: nodeConfig,
		}
	}
	return
}

func (host *EsxHost) VMwareAPIGetHostState(timeout int) (state string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	defer cancel()
	if host.HostClient != nil && host.HostSystem != nil {
		state, err = getHostState(ctx, host.HostSystem)
	} else {
		state = "connected"
	}
	return
}

func (host *EsxHost) VMwareAPIEnterMaintenanceMode(timeout int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	defer cancel()
	if host.HostClient != nil && host.HostSystem != nil {
		err = enterMaintenanceMode(ctx, host.HostSystem, timeout)
	}
	return
}

func (host *EsxHost) VMwareAPIExitMaintenanceMode(timeout int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	defer cancel()
	if host.HostClient != nil && host.HostSystem != nil {
		err = exitMaintenanceMode(ctx, host.HostSystem, timeout)
	}
	return
}

func (host *EsxHost) VMwareAPIIsInMaintenanceMode(timeout int) (inMaintenance bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	defer cancel()
	if host.HostClient != nil && host.HostSystem != nil {
		inMaintenance, err = isInMaintenanceMode(ctx, host.HostSystem)
	}
	return
}

func (host *EsxHost) VMwareAPIShutdownHost(timeout int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	defer cancel()
	if host.HostClient != nil && host.HostSystem != nil {
		err = shutdownHost(ctx, host.HostSystem)
	}
	return
}
