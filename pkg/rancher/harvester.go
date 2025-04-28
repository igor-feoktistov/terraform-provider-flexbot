package rancher

import (
	"fmt"
	"time"
	"math"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	harvesterWait4State = 5
)

// HarvesterNode is Harvester node definition
type HarvesterNode struct {
        HarvesterClient *HarvesterClient
	NodeConfig      *config.NodeConfig
	NodeID          string
}

func HarvesterAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *HarvesterNode, err error) {
        node = &HarvesterNode{
	        NodeConfig:       nodeConfig,
	}
	if meta.(*config.FlexbotConfig).RancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	rancherConfig := meta.(*config.FlexbotConfig).RancherConfig
	harvesterClient := NewHarvesterClient(
		rancherConfig.URL,
		&HarvesterClientOptions {
			BasicAuthToken:    rancherConfig.TokenKey,
			SSLVerify:         false,
			Debug:             false,
			Timeout:           60 * time.Second,
		},
	)
	harvesterClient.Retries = rancherConfig.Retries
	node.HarvesterClient = harvesterClient
	if waitForNode && meta.(*config.FlexbotConfig).WaitForNodeTimeout > 0 {
		var harvesterNode *corev1.Node
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*config.FlexbotConfig).WaitForNodeTimeout))
		for time.Now().Before(giveupTime) {
			var ready bool
			if ready, err = node.HarvesterClient.IsNodeReady(nodeConfig.Compute.HostName); err != nil {
				return
			}
			if ready {
				if _, harvesterNode, err = node.HarvesterClient.GetNode(nodeConfig.Compute.HostName); err == nil {
					node.NodeID = string(harvesterNode.ObjectMeta.UID)
				}
				return
			}
			time.Sleep(harvesterWait4State * time.Second)
		}
		err = fmt.Errorf("HarvesterAPIInitialize(): node \"%s\" is not ready after %d timeout", nodeConfig.Compute.HostName, meta.(*config.FlexbotConfig).WaitForNodeTimeout)
	}
	return
}

func (node *HarvesterNode) RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (err error) {
	var harvesterNode *corev1.Node
	if node.HarvesterClient == nil {
		return
	}
	if _, harvesterNode, err = node.HarvesterClient.GetNode(node.NodeConfig.Compute.HostName); err == nil {
		node.NodeID = string(harvesterNode.ObjectMeta.UID)
	}
        return
}

func (node *HarvesterNode) RancherAPINodeWaitUntilReady(timeout int) (err error) {
	if node.HarvesterClient == nil {
		return
	}
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		var ready bool
		if ready, err = node.HarvesterClient.IsNodeReady(node.NodeConfig.Compute.HostName); err != nil || ready {
			return
		}
		time.Sleep(harvesterWait4State * time.Second)
	}
	err = fmt.Errorf("RancherAPINodeWaitUntilReady(): node \"%s\" is not ready after %d timeout", node.NodeConfig.Compute.HostName, timeout)
	return
}

func (node *HarvesterNode) RancherAPINodeEnableMaintainanceMode(timeout int) (err error) {
	if node.HarvesterClient == nil {
		return
	}
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		var maintainance bool
		if maintainance, err = node.HarvesterClient.IsNodeInMaintainanceMode(node.NodeConfig.Compute.HostName); err != nil || maintainance {
			return
		}
		if err = node.HarvesterClient.NodeEnableMaintainanceMode(node.NodeConfig.Compute.HostName); err != nil {
			return
		}
		time.Sleep(harvesterWait4State * time.Second)
	}
	err = fmt.Errorf("RancherAPINodeEnableMaintainanceMode(): node \"%s\" is not in enabled maintainance mode after %d timeout", node.NodeConfig.Compute.HostName, timeout)
	return
}

func (node *HarvesterNode) RancherAPINodeDisableMaintainanceMode(timeout int) (err error) {
	if node.HarvesterClient == nil {
		return
	}
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		var maintainance bool
		if maintainance, err = node.HarvesterClient.IsNodeInMaintainanceMode(node.NodeConfig.Compute.HostName); err != nil || !maintainance {
			return
		}
		if err = node.HarvesterClient.NodeDisableMaintainanceMode(node.NodeConfig.Compute.HostName); err != nil {
			return
		}
		time.Sleep(harvesterWait4State * time.Second)
	}
	err = fmt.Errorf("RancherAPINodeDisableMaintainanceMode(): node \"%s\" is not in disabled maintainance mode after %d timeout", node.NodeConfig.Compute.HostName, timeout)
	return
}

func (node *HarvesterNode) RancherAPINodeGetState() (state string, err error) {
	return
}

func (node *HarvesterNode) RancherAPINodeWaitForState(state string, timeout int) (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
	if node.HarvesterClient != nil {
                giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
                        nextTimeout := int(math.Round(time.Until(giveupTime).Seconds()))
                        if nextTimeout > 0 {
	                        if err = node.RancherAPINodeWaitUntilReady(nextTimeout); err == nil {
			                time.Sleep(rancher2Wait4State * time.Second)
			        }
			}
		}
        }
        return
}

func (node *HarvesterNode) RancherAPINodeDelete() (err error) {
	if node.HarvesterClient == nil {
		return
	}
	err = node.HarvesterClient.DeleteNode(node.NodeConfig.Compute.HostName)
        return
}

func (node *HarvesterNode) RancherAPINodeForceDelete() (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeWaitUntilDeleted(timeout int) (err error) {
	return
}

func (node *HarvesterNode) RancherAPINodeCordon() (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeCordonDrain() (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeUncordon() (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
	return
}

func (node *HarvesterNode) RancherAPINodeGetLabels() (labels map[string]string, err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeGetTaints() (taints []corev1.Taint, err error) {
        return
}

func (node *HarvesterNode) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
        return
}

func (node *HarvesterNode) IsNodeControlPlane() (bool) {
        return false
}

func (node *HarvesterNode) IsNodeWorker() (bool) {
        return false
}

func (node *HarvesterNode) IsNodeEtcd() (bool) {
        return false
}

func (node *HarvesterNode) IsProviderRKE1() (bool) {
        return false
}

func (node *HarvesterNode) IsProviderRKE2() (bool) {
        return true
}

func (node *HarvesterNode) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
        return
}

func (node *HarvesterNode) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
        return
}
