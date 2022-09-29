package rancher

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

// RancherNode is rancher2 node definition
type Rancher2Node struct {
        RancherClient    *Rancher2Client
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *rancherManagementClient.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

func Rancher2APIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *Rancher2Node, err error) {
        node = &Rancher2Node{
	        NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if meta.(*config.FlexbotConfig).RancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	rancher2Config := &Rancher2Config{
	        RancherConfig: *(meta.(*config.FlexbotConfig).RancherConfig),
	}
	if err = rancher2Config.ManagementClient(); err != nil {
	        return
	}
	node.RancherClient = &rancher2Config.Client
	node.NodeDrainInput = rancher2Config.NodeDrainInput
        meta.(*config.FlexbotConfig).Sync.Lock()
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	if waitForNode && meta.(*config.FlexbotConfig).WaitForNodeTimeout > 0 {
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*config.FlexbotConfig).WaitForNodeTimeout))
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
			return
		}
		for time.Now().Before(giveupTime) {
			if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			        if !IsNotFound(err) {
				        return
				}
			}
			if len(node.NodeID) > 0 {
				if err = node.RancherClient.NodeWaitForState(node.NodeID, "active", int(math.Round(time.Until(giveupTime).Seconds()))); err == nil {
					node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
				}
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
	return
}

func (node *Rancher2Node) RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (err error) {
	if node.RancherClient != nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
	        network := d.Get("network").([]interface{})[0].(map[string]interface{})
                meta.(*config.FlexbotConfig).Sync.Unlock()
	        if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("rancherAPINodeGetID(): node %s not found", node.NodeConfig.Compute.HostName)
		}
	}
	return
}

func (node *Rancher2Node) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForState(node.ClusterID, state, timeout)
	}
	return
}

func (node *Rancher2Node) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForTransitioning(node.ClusterID, timeout)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil {
	        err = node.RancherClient.NodeWaitForState(node.NodeID, state, timeout)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
	if node.RancherClient != nil {
                giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
                        nextTimeout := int(math.Round(time.Until(giveupTime).Seconds()))
                        if nextTimeout > 0 {
	                        if err = node.RancherClient.NodeWaitForState(node.NodeID, "active", nextTimeout); err == nil {
			                time.Sleep(1 * time.Second)
			        }
			}
		}
        }
        return
}

func (node *Rancher2Node) RancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", Wait4ClusterStateTimeout); err == nil {
			err = node.RancherClient.NodeCordon(node.NodeID)
		}
	}
	return
}

func (node *Rancher2Node) RancherAPINodeCordonDrain() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", Wait4ClusterStateTimeout); err == nil {
			err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
		}
	}
	return
}

func (node *Rancher2Node) RancherAPINodeUncordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", Wait4ClusterStateTimeout); err == nil {
			if err = node.RancherClient.NodeWaitForState(node.NodeID, "active,drained,cordoned", Wait4NodeStateTimeout); err == nil {
				if err = node.RancherClient.NodeUncordon(node.NodeID); err == nil {
					err = node.RancherClient.NodeWaitForState(node.NodeID, "active", Wait4NodeStateTimeout)
				}
			}
		}
	}
	return
}

func (node *Rancher2Node) RancherAPINodeDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.DeleteNode(node.NodeID)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
	var computeB, storageB []byte
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		annotations := make(map[string]string)
		if len(node.NodeConfig.Compute.SpDn) > 0 && len(node.NodeConfig.Compute.BladeAssigned.Dn) > 0 {
			computeAnnotations := config.ComputeAnnotations{
				UcsmHost: node.NodeConfig.Compute.UcsmCredentials.Host,
				SpDn:     node.NodeConfig.Compute.SpDn,
				Blade:    node.NodeConfig.Compute.BladeAssigned,
			}
			if computeB, err = json.Marshal(computeAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(computeAnnotations): %s", err)
				return
			}
			annotations[config.NodeAnnotationCompute] = string(computeB)
		}
		if len(node.NodeConfig.Storage.SvmName) > 0 {
			storageAnnotations := config.StorageAnnotations{
				Svm:     node.NodeConfig.Storage.SvmName,
				Volume:  node.NodeConfig.Storage.VolumeName,
				Igroup:  node.NodeConfig.Storage.IgroupName,
				BootLun: node.NodeConfig.Storage.BootLun.Name,
				DataLun: node.NodeConfig.Storage.DataLun.Name,
				SeedLun: node.NodeConfig.Storage.SeedLun.Name,
			}
			storageAnnotations.BootImage.OsImage = node.NodeConfig.Storage.BootLun.OsImage.Name
			storageAnnotations.BootImage.SeedTemplate = node.NodeConfig.Storage.SeedLun.SeedTemplate.Name
			if storageB, err = json.Marshal(storageAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(storageAnnotations): %s", err)
				return
			}
			annotations[config.NodeAnnotationStorage] = string(storageB)
		}
		err = node.RancherClient.NodeSetAnnotationsLabelsTaints(node.NodeID, annotations, node.NodeConfig.Labels, node.NodeConfig.Taints)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeGetLabels() (labels map[string]string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		labels, err = node.RancherClient.NodeGetLabels(node.NodeID)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateLabels(node.NodeID, oldLabels, newLabels)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeGetTaints() (taints []rancherManagementClient.Taint, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		taints, err = node.RancherClient.NodeGetTaints(node.NodeID)
	}
	return
}

func (node *Rancher2Node) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateTaints(node.NodeID, oldTaints, newTaints)
	}
	return
}

func (node *Rancher2Node) IsNodeEtcd() (bool) {
        return node.NodeEtcd
}

func (node *Rancher2Node) IsNodeWorker() (bool) {
        return node.NodeWorker
}

func (node *Rancher2Node) IsNodeControlPlane() (bool) {
        return node.NodeControlPlane
}
