package rancher

import (
        "fmt"
        "time"
        "math"
        "encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
	NodeDrainInput   *rancherManagementClient.NodeDrainInput
	ClusterName      string
	ClusterID        string
	ClusterProvider  string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

func RkApiInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *RkApiNode, err error) {
	var machine *unstructured.Unstructured
        node = &RkApiNode{
	        NodeConfig:       nodeConfig,
		ClusterProvider:  "rke2",
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	rancherConfig := meta.(*config.FlexbotConfig).RancherConfig
	if rancherConfig == nil || !meta.(*config.FlexbotConfig).RancherApiEnabled {
		return
	}
	node.NodeDrainInput = rancherConfig.NodeDrainInput
        meta.(*config.FlexbotConfig).Sync.Lock()
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	node.ClusterName = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_name"].(string)
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
        meta.(*config.FlexbotConfig).Sync.Unlock()
	rkApiClient := &RkApiClient{}
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
	if machine, err = node.RancherClient.GetMachine(node.ClusterName, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		node.NodeID = getMapValue(machine.UnstructuredContent()["metadata"], "name").(string)
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
	}
	if waitForNode && meta.(*config.FlexbotConfig).WaitForNodeTimeout > 0 {
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*config.FlexbotConfig).WaitForNodeTimeout))
		if err = node.RancherClient.ClusterWaitForState(node.ClusterName, "active", meta.(*config.FlexbotConfig).WaitForNodeTimeout); err != nil {
			return
		}
		for time.Now().Before(giveupTime) {
			if machine, err = node.RancherClient.GetMachine(node.ClusterName, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			        if !node.RancherClient.IsMachineNotFound(err) {
				        return
				}
			}
			if err == nil {
				if err = node.RancherClient.MachineWaitForState(node.NodeID, "active", int(math.Round(time.Until(giveupTime).Seconds()))); err == nil {
					node.NodeID = getMapValue(machine.UnstructuredContent()["metadata"], "name").(string)
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
					return
				}
			}
			time.Sleep(rancher2Wait4State * time.Second)
		}
	        if err == nil && len(node.NodeID) == 0 {
	                err = fmt.Errorf("RkApiInitialize(): node with IP address %s is not found in the cluster", network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string))
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
	        if machine, err = node.RancherClient.GetMachine(node.ClusterName, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
			node.NodeID = getMapValue(machine.UnstructuredContent()["metadata"], "name").(string)
		} else {
			err = fmt.Errorf("rancherAPINodeGetID(): node %s not found", node.NodeConfig.Compute.HostName)
		}
	}
        return
}

func (node *RkApiNode) RancherAPIClusterWaitForState(state string, timeout int) (err error) {
        if node.RancherClient != nil {
                err = node.RancherClient.ClusterWaitForState(node.ClusterID, state, timeout)
        }
        return
}

func (node *RkApiNode) RancherAPIClusterWaitForTransitioning(timeout int) (err error) {
        return
}

func (node *RkApiNode) RancherAPINodeWaitForState(state string, timeout int) (err error) {
        return
}

func (node *RkApiNode) RancherAPINodeWaitForGracePeriod(timeout int) (err error) {
        return
}

func (node *RkApiNode) RancherAPINodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordon(node.NodeID)
	}
        return
}

func (node *RkApiNode) RancherAPINodeCordonDrain() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
	}
        return
}

func (node *RkApiNode) RancherAPINodeUncordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUncordon(node.NodeID)
	}
        return
}

func (node *RkApiNode) RancherAPINodeDelete() (err error) {
        return
}

func (node *RkApiNode) RancherAPINodeSetAnnotationsLabelsTaints() (err error) {
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

func (node *RkApiNode) RancherAPINodeGetLabels() (labels map[string]string, err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        labels, err = node.RancherClient.NodeGetLabels(node.NodeID)
	}
        return
}

func (node *RkApiNode) RancherAPINodeUpdateLabels(oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        err = node.RancherClient.NodeUpdateLabels(node.NodeID, oldLabels, newLabels)
	}
        return
}

func (node *RkApiNode) RancherAPINodeGetTaints() (taints []rancherManagementClient.Taint, err error) {
        var nodeTaints []v1.Taint
	if node.RancherClient != nil && len(node.NodeID) > 0 {
	        if nodeTaints, err = node.RancherClient.NodeGetTaints(node.NodeID); err == nil {
	                for _, taint := range nodeTaints {
		                taints = append(
                                        taints,
                                        rancherManagementClient.Taint{
                                                Key: taint.Key,
                                                Value: taint.Value,
                                                Effect: string(taint.Effect),
                                        })
	                }
	        }
	}
        return
}

func (node *RkApiNode) RancherAPINodeUpdateTaints(oldTaints []interface{}, newTaints []interface{}) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.NodeUpdateTaints(node.NodeID, oldTaints, newTaints)
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
