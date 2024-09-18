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
	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	rkeApiWait4State = 5
)

// RkApiNode is Rancher Kubernetes API node definition
type RkApiNode struct {
        RancherClient    *RkApiClient
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *config.NodeDrainInput
	ClusterName      string
	ClusterID        string
	NodeName         string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

func RkApiInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *RkApiNode, err error) {
	rancherConfig := meta.(*config.FlexbotConfig).RancherConfig
        node = &RkApiNode{
	        NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if rancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	node.NodeDrainInput = rancherConfig.NodeDrainInput
        meta.(*config.FlexbotConfig).Sync.Lock()
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	node.ClusterName = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_name"].(string)
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	rkApiClient := &RkApiClient{
		RancherConfig: rancherConfig,
	}
	k8sLocalClusterConfig := &rest.Config{
		Host: fmt.Sprintf("%s/k8s/clusters/local", rancherConfig.URL),
		TLSClientConfig: rest.TLSClientConfig{},
		BearerToken: rancherConfig.TokenKey,
		WarningHandler: rest.NoWarnings{},
	}
	if rkApiClient.LocalClusterClient, err = dynamic.NewForConfig(k8sLocalClusterConfig); err != nil {
		return
	}
	k8sDownstreamClusterConfig := &rest.Config{
		Host: fmt.Sprintf("%s/k8s/clusters/%s", rancherConfig.URL, node.ClusterID),
		TLSClientConfig: rest.TLSClientConfig{},
		BearerToken: rancherConfig.TokenKey,
		WarningHandler: rest.NoWarnings{},
	}
	if rkApiClient.DownstreamClusterClient, err = kubernetes.NewForConfig(k8sDownstreamClusterConfig); err != nil {
                return
        }
	node.RancherClient = rkApiClient
	node.RancherAPINodeGetID(d, meta)
	if waitForNode && meta.(*config.FlexbotConfig).WaitForNodeTimeout > 0 {
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*config.FlexbotConfig).WaitForNodeTimeout))
		if err = node.RancherClient.ClusterWaitForState(node.ClusterName, "active", meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
			return
		}
		for time.Now().Before(giveupTime) {
			if err = node.RancherAPINodeGetID(d, meta); err != nil {
			        if !node.RancherClient.IsMachineNotFound(err) {
				        return
				}
			}
			if err == nil {
				if err = node.RancherClient.MachineWaitForState(node.NodeID, "active", int(math.Round(time.Until(giveupTime).Seconds()))); err == nil {
					return
				}
			}
			time.Sleep(rancher2Wait4State * time.Second)
		}
	}
	return
}

func (node *RkApiNode) RancherAPINodeGetID(d *schema.ResourceData, meta interface{}) (err error) {
	var machine *unstructured.Unstructured
	if node.RancherClient != nil {
                meta.(*config.FlexbotConfig).Sync.Lock()
	        network := d.Get("network").([]interface{})[0].(map[string]interface{})
                meta.(*config.FlexbotConfig).Sync.Unlock()
	        if machine, err = node.RancherClient.GetMachineByNodeIp(node.ClusterName, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
	        	if machineName := getMapValue(machine.UnstructuredContent()["metadata"], "name"); machineName != nil {
				node.NodeID = machineName.(string)
			}
			if nodeRef := getMapValue(machine.UnstructuredContent()["status"], "nodeRef"); nodeRef != nil {
				if nodeName := getMapValue(nodeRef, "name"); nodeName != nil {
					node.NodeName = nodeName.(string)
				}
			}
			if labels := getMapValue(machine.UnstructuredContent()["metadata"], "labels"); labels != nil {
				if label, ok := labels.(map[string]interface{})["rke.cattle.io/worker-role"]; ok && label.(string) == "true" {
					node.NodeWorker = true
				}
				if label, ok := labels.(map[string]interface{})["rke.cattle.io/control-plane-role"]; ok && label.(string) == "true" {
					node.NodeControlPlane = true
				}
				if label, ok := labels.(map[string]interface{})["rke.cattle.io/etcd-role"]; ok && label.(string)== "true" {
					node.NodeEtcd = true
				}
			}
		} else {
			if node.RancherClient.IsMachineNotFound(err) {
				node.NodeID = ""
				node.NodeName = ""
			}
		}
	}
        return
}

func (node *RkApiNode) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
        if node.RancherClient != nil {
                err = node.RancherClient.ClusterWaitForState(node.ClusterName, state, timeout)
        }
        return
}

func (node *RkApiNode) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
        if node.RancherClient != nil {
                err = node.RancherClient.ClusterWaitForTransitioning(node.ClusterName, timeout)
        }
        return
}

func (node *RkApiNode) RancherAPINodeGetState() (state string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        state, err = node.RancherClient.GetMachineState(node.NodeID)
	}
        return
}

func (node *RkApiNode) RancherAPINodeWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.MachineWaitForState(node.NodeID, state, timeout)
	}
        return
}

func (node *RkApiNode) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
	if node.RancherClient != nil {
                giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
                        nextTimeout := int(math.Round(time.Until(giveupTime).Seconds()))
                        if nextTimeout > 0 {
	                        if err = node.RancherClient.MachineWaitForState(node.NodeID, "active", nextTimeout); err == nil {
			                time.Sleep(rkeApiWait4State * time.Second)
			        }
			}
		}
        }
        return
}

func (node *RkApiNode) RancherAPINodeWaitUntilDeleted(timeout int) (err error) {
	var machine *unstructured.Unstructured
	if node.RancherClient != nil {
		giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
		for time.Now().Before(giveupTime) {
			if machine, err = node.RancherClient.GetMachineByName(node.NodeID); err != nil {
				if node.RancherClient.IsMachineNotFound(err) {
					err = nil
				}
				return
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
		condition := "unknown"
		if machine != nil {
			condition = node.RancherClient.GetMachineCondition(machine)
		}
		err = fmt.Errorf("rk-api.RancherAPINodeWaitUntilDeleted(): wait exceeded timeout=%d: node condition: %s", timeout, condition)
	}
	return
}

func (node *RkApiNode) RancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
		err = node.RancherClient.NodeCordon(node.NodeName)
	}
        return
}

func (node *RkApiNode) RancherAPINodeCordonDrain() (err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
		err = node.RancherClient.NodeCordonDrain(node.NodeName, node.NodeDrainInput)
	}
        return
}

func (node *RkApiNode) RancherAPINodeUncordon() (err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
		err = node.RancherClient.NodeUncordon(node.NodeName)
	}
        return
}

func (node *RkApiNode) RancherAPINodeDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.DeleteMachine(node.NodeID)
	}
        return
}

func (node *RkApiNode) RancherAPINodeForceDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 && len(node.NodeName) > 0 {
		if _, err = node.RancherClient.GetMachineByName(node.NodeID); err == nil {
			err = node.RancherClient.NodeDelete(node.NodeName)
		} else {
			if node.RancherClient.IsMachineNotFound(err) {
				err = nil
			}
		}
	}
        return
}

func (node *RkApiNode) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
	var taints []v1.Taint
	var computeB, storageB []byte
	if node.RancherClient != nil && len(node.NodeName) > 0 {
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
		err = node.RancherClient.NodeSetAnnotationsLabelsTaints(node.NodeName, annotations, node.NodeConfig.Labels, taints)
	}
	return
}

func (node *RkApiNode) RancherAPINodeGetLabels() (labels map[string]string, err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
	        labels, err = node.RancherClient.NodeGetLabels(node.NodeName)
	}
        return
}

func (node *RkApiNode) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
	        err = node.RancherClient.NodeUpdateLabels(node.NodeName, oldLabels, newLabels)
	}
        return
}

func (node *RkApiNode) RancherAPINodeGetTaints() (taints []v1.Taint, err error) {
	var nodeTaints []v1.Taint
	if node.RancherClient != nil && len(node.NodeName) > 0 {
		if nodeTaints, err = node.RancherClient.NodeGetTaints(node.NodeName); err == nil {
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

func (node *RkApiNode) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeName) > 0 {
		err = node.RancherClient.NodeUpdateTaints(node.NodeName, oldTaints, newTaints)
	}
        return
}

func (node *RkApiNode) IsNodeControlPlane() (bool) {
        return node.NodeControlPlane
}

func (node *RkApiNode) IsNodeWorker() (bool) {
        return node.NodeWorker
}

func (node *RkApiNode) IsNodeEtcd() (bool) {
        return node.NodeEtcd
}

func (node *RkApiNode) IsProviderRKE1() (bool) {
        return false
}

func (node *RkApiNode) IsProviderRKE2() (bool) {
        return true
}
