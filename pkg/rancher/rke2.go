package rancher

import (
        "fmt"
        "time"
        "math"
        "encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Rke2Node is Rancher RKE2 node definition
type Rke2Node struct {
        RancherClient    *Rke2Client
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *config.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}


func Rke2APIInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *Rke2Node, err error) {
        node = &Rke2Node{
	        NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if meta.(*config.FlexbotConfig).RancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	rancherConfig := meta.(*config.FlexbotConfig).RancherConfig
	node.NodeDrainInput = meta.(*config.FlexbotConfig).RancherConfig.NodeDrainInput
        meta.(*config.FlexbotConfig).Sync.Lock()
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	rke2Config := &rest.Config{
	        Host: meta.(*config.FlexbotConfig).RancherConfig.URL,
                BearerToken: meta.(*config.FlexbotConfig).RancherConfig.TokenKey,
	        TLSClientConfig: rest.TLSClientConfig{
                        CAData: meta.(*config.FlexbotConfig).RancherConfig.ServerCAData,
                        CertData: meta.(*config.FlexbotConfig).RancherConfig.ClientCertData,
                        KeyData: meta.(*config.FlexbotConfig).RancherConfig.ClientKeyData,
	        },
	}
	rke2Client := &Rke2Client{
		RancherConfig: rancherConfig,
	}
	if rke2Client.Management, err = kubernetes.NewForConfig(rke2Config); err != nil {
	        return
	}
	node.RancherClient = rke2Client
	if node.NodeID, err = node.RancherClient.GetNodeName(network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	return
}

func (node *Rke2Node) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
        return
}

func (node *Rke2Node) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
        return
}

func (node *Rke2Node) RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (err error) {
	if node.RancherClient != nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
	        network := d.Get("network").([]interface{})[0].(map[string]interface{})
                meta.(*config.FlexbotConfig).Sync.Unlock()
	        if node.NodeID, err = node.RancherClient.GetNodeName(network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("rancherAPINodeGetID(): node %s not found", node.NodeConfig.Compute.HostName)
		}
	}
        return
}

func (node *Rke2Node) RancherAPINodeGetState() (state string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		var ready bool
		if ready, err = node.RancherClient.IsNodeReady(node.NodeID); err == nil {
			if ready {
				state = "active"
			} else {
				state = "notReady"
			}
		}
	}
        return
}

func (node *Rke2Node) RancherAPINodeWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		var nodeState string
		giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
			if nodeState, err = node.RancherAPINodeGetState(); err != nil {
				break
			}
			if nodeState == state {
				return
			}
			time.Sleep(rke2RetriesWait * time.Second)
		}
		if err == nil {
			state = nodeState
			err = fmt.Errorf("RancherAPINodeWaitForState(): wait exceeded timeout=%d: node state: %s", timeout, nodeState)
		}
	}
        return
}

func (node *Rke2Node) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
                giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
                        nextTimeout := int(math.Round(time.Until(giveupTime).Seconds()))
                        if nextTimeout > 0 {
	                        if err = node.RancherAPINodeWaitForState("active", nextTimeout); err == nil {
			                time.Sleep(rke2RetriesWait * time.Second)
			        }
			}
		}
        }
        return
}

func (node *Rke2Node) RancherAPINodeDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeDelete(node.NodeID)
	}
        return
}

func (node *Rke2Node) RancherAPINodeForceDelete() (err error) {
        return
}

func (node *Rke2Node) RancherAPINodeWaitUntilDeleted(timeout int) (err error) {
	return
}

func (node *Rke2Node) RancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordon(node.NodeID)
	}
        return
}

func (node *Rke2Node) RancherAPINodeCordonDrain() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
	}
        return
}

func (node *Rke2Node) RancherAPINodeUncordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUncordon(node.NodeID)
	}
        return
}

func (node *Rke2Node) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
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

func (node *Rke2Node) RancherAPINodeGetLabels() (labels map[string]string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        labels, err = node.RancherClient.NodeGetLabels(node.NodeID)
	}
        return
}

func (node *Rke2Node) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUpdateLabels(node.NodeID, oldLabels, newLabels)
	}
        return
}

func (node *Rke2Node) RancherAPINodeGetTaints() (taints []v1.Taint, err error) {
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

func (node *Rke2Node) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateTaints(node.NodeID, oldTaints, newTaints)
	}
        return
}

func (node *Rke2Node) IsNodeControlPlane() (bool) {
        return node.NodeControlPlane
}

func (node *Rke2Node) IsNodeWorker() (bool) {
        return node.NodeWorker
}

func (node *Rke2Node) IsNodeEtcd() (bool) {
        return node.NodeEtcd
}

func (node *Rke2Node) IsProviderRKE1() (bool) {
        return false
}

func (node *Rke2Node) IsProviderRKE2() (bool) {
        return true
}
