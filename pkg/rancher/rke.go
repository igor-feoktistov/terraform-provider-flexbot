package rancher

import (
        "fmt"
        "encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// RkeNode is Rancher RKE node definition
type RkeNode struct {
        RancherClient    *RkeClient
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *config.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}


func RkeAPIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *RkeNode, err error) {
        node = &RkeNode{
	        NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if meta.(*config.FlexbotConfig).RancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	node.NodeDrainInput = meta.(*config.FlexbotConfig).RancherConfig.NodeDrainInput
        meta.(*config.FlexbotConfig).Sync.Lock()
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	rkeConfig := &rest.Config{
	        Host: meta.(*config.FlexbotConfig).RancherConfig.URL,
                BearerToken: meta.(*config.FlexbotConfig).RancherConfig.TokenKey,
	        TLSClientConfig: rest.TLSClientConfig{
                        CAData: meta.(*config.FlexbotConfig).RancherConfig.ServerCAData,
                        CertData: meta.(*config.FlexbotConfig).RancherConfig.ClientCertData,
                        KeyData: meta.(*config.FlexbotConfig).RancherConfig.ClientKeyData,
	        },
	}
	rkeClient := &RkeClient{}
	if rkeClient.Management, err = kubernetes.NewForConfig(rkeConfig); err != nil {
	        return
	}
	node.RancherClient = rkeClient
	if node.NodeID, err = node.RancherClient.GetNode(network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	return
}

func (node *RkeNode) RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (err error) {
	if node.RancherClient != nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
	        network := d.Get("network").([]interface{})[0].(map[string]interface{})
                meta.(*config.FlexbotConfig).Sync.Unlock()
	        if node.NodeID, err = node.RancherClient.GetNode(network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("rancherAPINodeGetID(): node %s not found", node.NodeConfig.Compute.HostName)
		}
	}
        return
}

func (node *RkeNode) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
        return
}

func (node *RkeNode) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
        return
}

func (node *RkeNode) RancherAPINodeWaitUntilDeleted(timeout int) (err error) {
	return
}

func (node *RkeNode) RancherAPINodeWaitForState(state string, timeout int) (err error) {
        return
}

func (node *RkeNode) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
        return
}

func (node *RkeNode) RancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordon(node.NodeID)
	}
        return
}

func (node *RkeNode) RancherAPINodeCordonDrain() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
	}
        return
}

func (node *RkeNode) RancherAPINodeUncordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUncordon(node.NodeID)
	}
        return
}

func (node *RkeNode) RancherAPINodeDelete() (err error) {
        return
}

func (node *RkeNode) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
	var taints []v1.Taint
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
				SeedLun: node.NodeConfig.Storage.SeedLun.Name,
			}
			storageAnnotations.BootImage.OsImage = node.NodeConfig.Storage.BootLun.OsImage.Name
			storageAnnotations.BootImage.SeedTemplate = node.NodeConfig.Storage.SeedLun.SeedTemplate.Name
			if node.NodeConfig.Storage.DataLun.Size > 0 {
				storageAnnotations.DataLun = node.NodeConfig.Storage.DataLun.Name
			}
			if node.NodeConfig.Storage.DataNvme.Size > 0 {
				storageAnnotations.DataNvme.Namespace = node.NodeConfig.Storage.DataNvme.Namespace
				storageAnnotations.DataNvme.Subsystem = node.NodeConfig.Storage.DataNvme.Subsystem
			}
			if storageB, err = json.Marshal(storageAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(storageAnnotations): %s", err)
				return
			}
			annotations[config.NodeAnnotationStorage] = string(storageB)
		}
                for _, taint := range node.NodeConfig.Taints {
                        taints = append(
	                        taints,
	                        v1.Taint{
	                                Key: taint.Key,
	                                Value: taint.Value,
	                                Effect: v1.TaintEffect(taint.Effect),
	                        })
                }
		err = node.RancherClient.NodeSetAnnotationsLabelsTaints(node.NodeID, annotations, node.NodeConfig.Labels, taints)
	}
	return
}

func (node *RkeNode) RancherAPINodeGetLabels() (labels map[string]string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        labels, err = node.RancherClient.NodeGetLabels(node.NodeID)
	}
        return
}

func (node *RkeNode) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUpdateLabels(node.NodeID, oldLabels, newLabels)
	}
        return
}

func (node *RkeNode) RancherAPINodeGetTaints() (taints []v1.Taint, err error) {
	var nodeTaints []v1.Taint
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if nodeTaints, err = node.RancherClient.NodeGetTaints(node.NodeID); err == nil {
			for _, taint := range nodeTaints {
				taints = append(
					taints,
					v1.Taint{
						Key: taint.Key,
						Value: taint.Value,
						Effect: taint.Effect,
					})
			}
		}
	}
        return
}

func (node *RkeNode) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateTaints(node.NodeID, oldTaints, newTaints)
	}
        return
}

func (node *RkeNode) IsNodeControlPlane() (bool) {
        return node.NodeControlPlane
}

func (node *RkeNode) IsNodeWorker() (bool) {
        return node.NodeWorker
}

func (node *RkeNode) IsNodeEtcd() (bool) {
        return node.NodeEtcd
}

func (node *RkeNode) IsProviderRKE1() (bool) {
        return true
}

func (node *RkeNode) IsProviderRKE2() (bool) {
        return false
}
