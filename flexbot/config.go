package flexbot

import (
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/rancher"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

// UpdateManager ensures serialization while node maintenance
type UpdateManager struct {
	Sync      sync.Mutex
	LastError error
}

// FlexbotConfig is main provider configration
type FlexbotConfig struct {
	Sync                  *sync.Mutex
	FlexbotProvider       *schema.ResourceData
	RancherApiEnabled     bool
	RancherConfig         *rancher.Config
	RancherNodeDrainInput *rancherManagementClient.NodeDrainInput
	NodeGraceTimeout      int
	WaitForNodeTimeout    int
	UpdateManager         UpdateManager
	NodeConfig            map[string]*config.NodeConfig
}

// UpdateManagerAcquire acquires UpdateManager
func (c *FlexbotConfig) UpdateManagerAcquire() error {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.Sync.Lock()
		return c.UpdateManager.LastError
	}
	return nil
}

// UpdateManagerSetError sets error in UpdateManager
func (c *FlexbotConfig) UpdateManagerSetError(err error) {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.LastError = err
	}
}

// UpdateManagerRelease releases UpdateManager
func (c *FlexbotConfig) UpdateManagerRelease() {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.Sync.Unlock()
	}
}
