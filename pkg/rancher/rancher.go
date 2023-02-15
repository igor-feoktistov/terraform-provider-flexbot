package rancher

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

// Default timeouts
const (
	Wait4ClusterStateTimeout = 1800
	Wait4NodeStateTimeout    = 600
)

type RancherNode interface {
        RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (error)
        RancherAPIClusterWaitForState(state string, timeout int) (error)
        RancherAPIClusterWaitForTransitioning(timeout int) (error)
        RancherAPINodeWaitForState(state string, timeout int) (error)
        RancherAPINodeWaitForGracePeriod(timeout int) (error)
        RancherAPINodeCordon() (error)
        RancherAPINodeCordonDrain() (error)
        RancherAPINodeUncordon() (error)
        RancherAPINodeDelete() (error)
        RancherAPINodeSetAnnotationsLabelsTaints() (error)
        RancherAPINodeGetLabels() (map[string]string, error)
        RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (error)
        RancherAPINodeGetTaints() ([]rancherManagementClient.Taint, error)
        RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (error)
        IsNodeControlPlane() (bool)
        IsNodeWorker() (bool)
        IsNodeEtcd() (bool)
        IsProviderRKE1() (bool)
        IsProviderRKE2() (bool)
}

func RancherAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node RancherNode, err error) {
	if meta.(*config.FlexbotConfig).RancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
	        node = &Rancher2Node{
		        NodeConfig:       nodeConfig,
	        }
		return
	}
	switch meta.(*config.FlexbotConfig).RancherConfig.Provider {
	case "rancher2":
		if node, err = Rancher2APIInitialize(d, meta, nodeConfig, waitForNode); err != nil {
                        err = fmt.Errorf("Rancher2APIInitialize(): error: %s", err)
		}
	case "rke":
	        if node, err = RkeAPIInitialize(d, meta, nodeConfig, waitForNode); err != nil {
                        err = fmt.Errorf("RkeAPIInitialize(): error: %s", err)
                }
	default:
		err = fmt.Errorf("RancherAPIInitialize(): rancher API provider %s is not implemented", meta.(*config.FlexbotConfig).RancherConfig.Provider)
	}
	return
}

func DiscoverNode(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
        var node RancherNode
        var labels map[string]string
        var nodeTaints, declaredTaints []rancherManagementClient.Taint
        if node, err = RancherAPIInitialize(d, meta, nodeConfig, false); err == nil {
                if labels, err = node.RancherAPINodeGetLabels(); err == nil {
                        nodeTaints, err = node.RancherAPINodeGetTaints()
                }
                if err != nil {
                        err = fmt.Errorf("rancher.DiscoverNode(): error: %s", err)
                        return
                }
                if labels != nil {
                        for key := range nodeConfig.Labels {
                                if _, ok := labels[key]; !ok {
                                        delete(nodeConfig.Labels, key)
                                }
                        }
                }
                declaredTaints = nodeConfig.Taints
                nodeConfig.Taints = make([]rancherManagementClient.Taint, 0)
                if nodeTaints != nil {
	                for _, declaredTaint := range declaredTaints {
	                        for _, nodeTaint := range nodeTaints {
	                                if nodeTaint.Key == declaredTaint.Key && nodeTaint.Value == declaredTaint.Value && nodeTaint.Effect == declaredTaint.Effect {
	                                        nodeConfig.Taints = append(nodeConfig.Taints, declaredTaint)
	                                        break
	                                }
	                        }
	                }
	        }
        }
        return
}