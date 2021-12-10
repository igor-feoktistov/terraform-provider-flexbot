package flexbot

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/go-ucsm-sdk/util"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/rancher"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

// Default timeouts
const (
	Wait4ClusterStateTimeout = 1800
	Wait4NodeStateTimeout    = 600
)

// Default annotation names
const (
	NodeAnnotationCompute = "flexpod-compute"
	NodeAnnotationStorage = "flexpod-storage"
)

// RancherNode is node definition
type RancherNode struct {
	RancherClient    *rancher.Client
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *rancherManagementClient.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

// ComputeAnnotations is node annotations for compute
type ComputeAnnotations struct {
	UcsmHost string         `yaml:"ucsmHost" json:"ucsmHost"`
	SpDn     string         `yaml:"spDn" json:"spDn"`
	Blade    util.BladeSpec `yaml:"blade" json:"blade"`
}

// StorageAnnotations is node annotations for storage
type StorageAnnotations struct {
	BootImage struct {
		OsImage      string `yaml:"osImage" json:"osImage"`
		SeedTemplate string `yaml:"seedTemplate" json:"seedTemplate"`
	} `yaml:"bootImage" json:"bootImage"`
	Svm     string `yaml:"svm" json:"svm"`
	Volume  string `yaml:"volume" json:"volume"`
	Igroup  string `yaml:"igroup" json:"igroup"`
	BootLun string `yaml:"bootLun" json:"bootLun"`
	DataLun string `yaml:"dataLun" json:"dataLun"`
	SeedLun string `yaml:"seedLun" json:"seedLun"`
}

func rancherAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *RancherNode, err error) {
	p := meta.(*FlexbotConfig).FlexbotProvider
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	node = &RancherNode{
		NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if meta.(*FlexbotConfig).RancherConfig == nil || !meta.(*FlexbotConfig).RancherApiEnabled {
		return
	}
	node.RancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	node.NodeDrainInput = meta.(*FlexbotConfig).RancherConfig.NodeDrainInput
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
	if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	if waitForNode && meta.(*FlexbotConfig).WaitForNodeTimeout > 0 {
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*FlexbotConfig).WaitForNodeTimeout))
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", meta.(*FlexbotConfig).WaitForNodeTimeout); err != nil {
			return
		}
		for time.Now().Before(giveupTime) {
			if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			        if !rancher.IsNotFound(err) {
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

func (node *RancherNode) rancherAPIClusterWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForState(node.ClusterID, state, timeout)
	}
	return
}

func (node *RancherNode) rancherAPIClusterWaitForTransitioning(timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForTransitioning(node.ClusterID, timeout)
	}
	return
}

func (node *RancherNode) rancherAPINodeGetID(d *schema.ResourceData) (err error) {
	if node.RancherClient != nil {
	        network := d.Get("network").([]interface{})[0].(map[string]interface{})
	        if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("rancherAPINodeGetID(): node %s not found", node.NodeConfig.Compute.HostName)
		}
	}
	return
}

func (node *RancherNode) rancherAPINodeWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil {
	        err = node.RancherClient.NodeWaitForState(node.NodeID, state, timeout)
	}
	return
}

func (node *RancherNode) rancherAPINodeWaitForGracePeriod(timeout int) (err error) {
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

func (node *RancherNode) rancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", Wait4ClusterStateTimeout); err == nil {
			err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
		}
	}
	return
}

func (node *RancherNode) rancherAPINodeUncordon() (err error) {
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

func (node *RancherNode) rancherAPINodeDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.DeleteNode(node.NodeID)
	}
	return
}

func (node *RancherNode) rancherAPINodeSetAnnotationsLabels() (err error) {
	var computeB, storageB []byte
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		annotations := make(map[string]string)
		if len(node.NodeConfig.Compute.SpDn) > 0 && len(node.NodeConfig.Compute.BladeAssigned.Dn) > 0 {
			computeAnnotations := ComputeAnnotations{
				UcsmHost: node.NodeConfig.Compute.UcsmCredentials.Host,
				SpDn:     node.NodeConfig.Compute.SpDn,
				Blade:    node.NodeConfig.Compute.BladeAssigned,
			}
			if computeB, err = json.Marshal(computeAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(computeAnnotations): %s", err)
				return
			}
			annotations[NodeAnnotationCompute] = string(computeB)
		}
		if len(node.NodeConfig.Storage.SvmName) > 0 {
			storageAnnotations := StorageAnnotations{
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
			annotations[NodeAnnotationStorage] = string(storageB)
		}
		err = node.RancherClient.NodeSetAnnotationsLabels(node.NodeID, annotations, node.NodeConfig.Labels)
	}
	return
}

func (node *RancherNode) rancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateLabels(node.NodeID, oldLabels, newLabels)
	}
	return
}
