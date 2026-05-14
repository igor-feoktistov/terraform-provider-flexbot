package vmware

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

type VMwareAPI interface {
        VMwareAPIEnterMaintenanceMode(timeout int) (error)
        VMwareAPIExitMaintenanceMode(timeout int) (error)
        VMwareAPIIsInMaintenanceMode(timeout int) (bool, error)
        VMwareAPIShutdownHost(timeout int) (error)
        VMwareAPIGetHostState(timeout int) (string, error)
}

func VMwareAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (vmwareAPI VMwareAPI, err error) {
	if meta.(*config.FlexbotConfig).VMwareConfig == nil || !meta.(*config.FlexbotConfig).VMwareApiEnabled {
		return
	}
	switch meta.(*config.FlexbotConfig).VMwareConfig.Provider {
	case "host":
		if vmwareAPI, err = EsxHostAPIInitialize(d, meta, nodeConfig); err != nil {
                        err = fmt.Errorf("EsxHostAPIInitialize(): error: %s", err)
                }
	default:
		err = fmt.Errorf("VMwareAPIInitialize(): VMware API provider %s is not implemented", meta.(*config.FlexbotConfig).VMwareConfig.Provider)
	}
	return
}
