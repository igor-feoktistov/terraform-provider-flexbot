package rancher

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	rancher2ClientAPIVersion = "/v3"
	rancher2RetriesWait = 5
	rancher2StabilizeWait = 3
	rancher2StabilizeMax = 10
)

// Rancher2Client is rancher2 client
type Rancher2Client struct {
	Management *managementClient.Client
	MachineClient dynamic.NamespaceableResourceInterface
	Retries int
}

// Rancher2Config is rancher2 client configuration
type Rancher2Config struct {
        config.RancherConfig
	Client Rancher2Client
}

// RootURL gets root URL
func RootURL(url string) string {
	NormalizeURL(url)
	url = strings.TrimSuffix(url, "/v3")
	return url
}

// NewListOpts creates ListOpts
func NewListOpts(filters map[string]interface{}) *types.ListOpts {
	listOpts := clientbase.NewListOpts()
	if filters != nil {
		listOpts.Filters = filters
	}
	return listOpts
}

// IsNotFound checks NotFound in return
func IsNotFound(err error) bool {
	return clientbase.IsNotFound(err)
}

// IsForbidden checks Forbidden in return
func IsForbidden(err error) bool {
	apiError, ok := err.(*clientbase.APIError)
	if !ok {
		return false
	}
	return apiError.StatusCode == http.StatusForbidden
}

// InitializeClient initializes Rancher Management Client
func (c *Rancher2Config) InitializeClient() (err error) {
	c.Sync.Lock()
	defer c.Sync.Unlock()
	if c.Client.Management != nil {
		return
	}
	for i := 0; i <= c.Retries; i++ {
	        var resp []byte
		resp, err = DoGet(RootURL(c.URL) + rancherReadyRequest, "", "", "", string(c.ServerCAData), c.Insecure)
		if err == nil && rancherReadyResponse == string(resp) {
			break
		}
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	if err != nil {
	        err = fmt.Errorf("rancher.InitializeClient(): rancher is not ready after %d attempts in %d seconds, last error: %s", c.Retries, rancher2RetriesWait * c.Retries, err)
		return
	}
	c.URL = NormalizeURL(c.URL)
	options := &clientbase.ClientOpts{
		URL:      c.URL,
		TokenKey: c.TokenKey,
		CACerts:  string(c.ServerCAData),
		Insecure: c.Insecure,
	}
	options.URL = options.URL + rancher2ClientAPIVersion
	if c.Client.Management, err = managementClient.NewClient(options); err != nil {
                fmt.Errorf("rancher.InitializeClient(): failed to create rancher management client: %s", err)
		return
	}
	c.Client.Retries = c.Retries
	restConfig := &rest.Config{
		Host: c.URL + "/k8s/clusters/local",
                TLSClientConfig: rest.TLSClientConfig{},
                BearerToken: c.TokenKey,
                WarningHandler: rest.NoWarnings{},
        }
	var dynamicClient dynamic.Interface
	if dynamicClient, err = dynamic.NewForConfig(restConfig); err != nil {
		fmt.Errorf("rancher.InitializeClient(): failed to create dynamic client: %s", err)
		return
	}
	c.Client.MachineClient = dynamicClient.Resource(schema.GroupVersionResource{Group: CAPI_Group, Version: CAPI_Version, Resource: CAPI_MachineResource})
	return
}

// Retry Rancher API probe number of "Retries" attempts or until ready
func (client *Rancher2Client) isRancherReady() (err error) {
	var resp []byte
	url := RootURL(client.Management.Opts.URL) + rancherReadyRequest
	for retry := 0; retry < client.Retries; retry++ {
	        for stabilize := 0; stabilize < rancher2StabilizeMax; stabilize++ {
		        resp, err = DoGet(url, "", "", "", client.Management.Opts.CACerts, client.Management.Opts.Insecure)
		        if err == nil && rancherReadyResponse == string(resp) {
		                time.Sleep(rancher2StabilizeWait * time.Second)
		        } else {
		                break
		        }
		}
		if err == nil && rancherReadyResponse == string(resp) {
		        return
                }
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	return fmt.Errorf("rancher2-client.isRancherReady(): rancher is not ready after %d attempts in %d seconds, last error: %s", client.Retries, rancher2RetriesWait * client.Retries, err)
}

// Resilient to transient errors version of Cluster.List
func (client *Rancher2Client) GetClusterList(opts *types.ListOpts) (clusters *managementClient.ClusterCollection, err error) {
        if clusters, err = client.Management.Cluster.List(opts); err != nil {
                if err = client.isRancherReady(); err == nil {
                        clusters, err = client.Management.Cluster.List(opts)
                }
        }
        return
}

// Resilient to transient errors version of Node.List
func (client *Rancher2Client) GetNodeList(opts *types.ListOpts) (nodes *managementClient.NodeCollection, err error) {
        if nodes, err = client.Management.Node.List(opts); err != nil {
                if err = client.isRancherReady(); err == nil {
                        nodes, err = client.Management.Node.List(opts)
                }
        }
        return
}

// Resilient to transient errors version of Cluster.ByID
func (client *Rancher2Client) GetClusterByID(clusterID string) (cluster *managementClient.Cluster, err error) {
        if cluster, err = client.Management.Cluster.ByID(clusterID); err != nil {
                if err = client.isRancherReady(); err == nil {
                        cluster, err = client.Management.Cluster.ByID(clusterID)
                }
        }
        return
}

// Resilient to transient errors version of Node.ByID
func (client *Rancher2Client) GetNodeByID(nodeID string) (node *managementClient.Node, err error) {
        if node, err = client.Management.Node.ByID(nodeID); err != nil {
                if err = client.isRancherReady(); err == nil {
                        node, err = client.Management.Node.ByID(nodeID)
                }
        }
        return
}

// GetNode gets Rancher node by cluster ID and node IP address
func (client *Rancher2Client) GetNodeByAddr(clusterID string, nodeIPAddr string) (cluster *managementClient.Cluster, node *managementClient.Node, nodeID string, err error) {
	var clusters *managementClient.ClusterCollection
	var nodes *managementClient.NodeCollection
	filters := map[string]interface{}{
		"id": clusterID,
	}
	clusters, err = client.GetClusterList(NewListOpts(filters))
	if err == nil && len(clusters.Data) > 0 {
	        cluster = &clusters.Data[0]
		filters := map[string]interface{}{
			"clusterId": clusterID,
			"ipAddress": nodeIPAddr,
		}
		nodes, err = client.GetNodeList(NewListOpts(filters))
		if err == nil && len(nodes.Data) > 0 {
			nodeID = nodes.Data[0].ID
			node = &nodes.Data[0]
		}
	}
	if err != nil {
		err = fmt.Errorf("rancher2-client.GetNodeByAddr() error: %s", err)
	}
	return
}

// GetNodeRole gets Rancher node role
func (client *Rancher2Client) GetNodeRole(nodeID string) (controlplane bool, etcd bool, worker bool, err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.GetNodeRole() error: %s", err)
		return
	}
	return node.ControlPlane, node.Etcd, node.Worker, nil
}

// GetNodeMachine gets Rancher node machine resource
func (client *Rancher2Client) GetNodeMachine(node *managementClient.Node) (machine *unstructured.Unstructured, err error) {
	var machineUnstructured *unstructured.Unstructured
	var cluster_namespace, machine_name string
	cluster_namespace = node.Annotations["cluster.x-k8s.io/cluster-namespace"]
	machine_name = node.Annotations["cluster.x-k8s.io/machine"]
	if len(cluster_namespace) > 0 && len(machine_name) > 0 {
		if machineUnstructured, err = client.MachineClient.Namespace(cluster_namespace).Get(context.Background(), machine_name, metav1.GetOptions{}); err == nil {
			machine = machineUnstructured
		} else {
			fmt.Errorf("rancher2-client.GetNodeMachine().Get() error: %s", err)
        	}
	} else {
		var machineList *unstructured.UnstructuredList
		if machineList, err = client.MachineClient.List(context.Background(), metav1.ListOptions{}); err == nil {
			for _, item := range machineList.Items {
				u := item.UnstructuredContent()
				labels, ok := u["metadata"].(map[string]interface{})["labels"]
				if ok && labels.(map[string]interface{})["rke.cattle.io/node-name"] != nil && labels.(map[string]interface{})["rke.cattle.io/node-name"].(string) == node.NodeName {
					machine = &item
					break
				}
			}
		} else {
			fmt.Errorf("rancher2-client.GetNodeMachine().List() error: %s", err)
		}
	}
	return
}

// ClusterWaitForState waits until cluster is in specified state
func (client *Rancher2Client) ClusterWaitForState(clusterID string, states string, timeout int) (err error) {
	var cluster *managementClient.Cluster
	var clusterLastState string
	var settleDown int
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if cluster, err = client.GetClusterByID(clusterID); err != nil {
			if IsNotFound(err) {
				err = fmt.Errorf("rancher2-client.ClusterWaitForState().GetClusterByID(): cluster not found")
			}
			if IsForbidden(err) {
				err = fmt.Errorf("rancher2-client.ClusterWaitForState().GetClusterByID(): access denied")
			}
			return
		}
		if clusterLastState != cluster.State {
	                settleDown = 10
		}
		for _, state := range strings.Split(states, ",") {
			if cluster.State == state {
			        if settleDown == 0 {
				        return
				} else {
				        settleDown--
				        break
				}
			}
		}
		clusterLastState = cluster.State
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	for _, state := range strings.Split(states, ",") {
	        if clusterLastState == state {
	                return
	        }
	}
	err = fmt.Errorf("rancher2-client.ClusterWaitForState(): wait for cluster state exceeded timeout=%d: expected states=%s, last state=%s", timeout, states, clusterLastState)
	return
}

// ClusterWaitForTransitioning waits until cluster enters transitioning state
func (client *Rancher2Client) ClusterWaitForTransitioning(clusterID string, timeout int) (err error) {
	var cluster *managementClient.Cluster
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if cluster, err = client.GetClusterByID(clusterID); err != nil {
			if IsNotFound(err) {
				err = fmt.Errorf("rancher2-client.ClusterWaitForTransitioning().GetClusterByID(): cluster not found")
			}
			if IsForbidden(err) {
				err = fmt.Errorf("rancher2-client.ClusterWaitForTransitioning().GetClusterByID(): access denied")
			}
			return
		}
		if (cluster.State == "updating" || cluster.State == "upgrading") && (cluster.Transitioning == "yes" || cluster.Transitioning == "error") {
		        return
		}
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	err = fmt.Errorf("rancher2-client.ClusterWaitForTransitioning(): wait for cluster transitioning exceeded timeout=%d", timeout)
	return
}

// NodeGetState gets Rancher node state
func (client *Rancher2Client) NodeGetState(nodeID string) (state string, err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeGetState() error: %s", err)
	} else {
		state = node.State
	}
	return
}

// NodeWaitForState waits until Rancher node in specified state
func (client *Rancher2Client) NodeWaitForState(nodeID string, states string, timeout int) (err error) {
	var node *managementClient.Node
	var nodeLastState string
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if node, err = client.GetNodeByID(nodeID); err != nil {
			err = fmt.Errorf("rancher-client.NodeWaitForState().GetNodeByID() error: %s", err)
			return
		}
		for _, state := range strings.Split(states, ",") {
			if node.State == state {
				return
			}
		}
		nodeLastState = node.State
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	err = fmt.Errorf("rancher2-client.NodeWaitForState(): wait for node state exceeded timeout=%d: expected states=%s, last state=%s", timeout, states, nodeLastState)
	return
}

// NodeCordon cordon Rancher node
func (client *Rancher2Client) NodeCordon(nodeID string) (err error) {
	var node *managementClient.Node
	var ok bool
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeCordon().GetNodeByID() error: %s", err)
		return
	}
	_, ok = node.Actions["cordon"]
	if ok {
                err = client.Management.Node.ActionCordon(node)
	}
	if err != nil {
		err = fmt.Errorf("rancher2-client.NodeCordon() error: %s", err)
	}
	return
}

// NodeCordonDrain cordon/drain Rancher node
func (client *Rancher2Client) NodeCordonDrain(nodeID string, nodeDrainInput *managementClient.NodeDrainInput) (err error) {
	var node *managementClient.Node
	var ok bool
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeCordonDrain().GetNodeByID() error: %s", err)
		return
	}
	_, ok = node.Actions["cordon"]
	if ok {
		if err = client.Management.Node.ActionCordon(node); err != nil {
			return
		}
	}
	_, ok = node.Actions["drain"]
	if ok {
		if err = client.Management.Node.ActionDrain(node, nodeDrainInput); err == nil {
			if err = client.NodeWaitForState(nodeID, "draining,drained", int(nodeDrainInput.Timeout+nodeDrainInput.GracePeriod)); err == nil {
				if err = client.NodeWaitForState(nodeID, "drained", int(nodeDrainInput.Timeout+nodeDrainInput.GracePeriod)); err != nil {
				        client.Management.Node.ActionDrain(node, nodeDrainInput)
				        client.NodeWaitForState(nodeID, "drained", int(nodeDrainInput.Timeout))
				        return nil
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("rancher2-client.NodeCordonDrain() error: %s", err)
	}
	return
}

// NodeUncordon uncordon Rancher node
func (client *Rancher2Client) NodeUncordon(nodeID string) (err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeUncordon().GetNodeByID() error: %s", err)
		return
	}
	_, ok := node.Actions["uncordon"]
	if ok {
		err = client.Management.Node.ActionUncordon(node)
	}
	if err != nil {
		err = fmt.Errorf("rancher2-client.NodeUncordon() error: %s", err)
	}
	return
}

// DeleteNode deletes Rancher node and node machine
func (client *Rancher2Client) DeleteNode(nodeID string) (err error) {
	var node *managementClient.Node
	var machine *unstructured.Unstructured
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.DeleteNode().GetNodeByID() error: %s", err)
		return
	}
	if machine, err = client.GetNodeMachine(node); err != nil {
		err = fmt.Errorf("rancher2-client.DeleteNode().GetNodeMachine() error: %s", err)
		return
	}
	if err = client.Management.Node.Delete(node); err != nil {
		err = fmt.Errorf("rancher2-client.DeleteNode() error: %s", err)
		return
	}
	if machine != nil {
		u := machine.UnstructuredContent()
		machineName := GetMapString(u["metadata"], "name")
		machineNamespace := GetMapString(u["metadata"], "namespace")
		if len(machineName) > 0 && len(machineNamespace) > 0 {
			client.MachineClient.Namespace(machineNamespace).Delete(context.Background(), machineName, metav1.DeleteOptions{})
		}
	}
	return
}

// NodeSetAnnotationsLabelsTaints sets Rancher node annotations, labels, and taints
func (client *Rancher2Client) NodeSetAnnotationsLabelsTaints(nodeID string, annotations map[string]string, labels map[string]string, taints []managementClient.Taint) (err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeSetAnnotationsLabelsTaints().GetNodeByID() error: %s", err)
		return
	}
	for key, elem := range annotations {
		node.Annotations[key] = elem
	}
	for key, elem := range labels {
		node.Labels[key] = elem
	}
	for _, taint := range taints {
	        matched := false
	        for _, nodeTaint := range node.Taints {
	                if taint.Key == nodeTaint.Key && taint.Value == nodeTaint.Value && taint.Effect == nodeTaint.Effect {
	                        matched = true
	                }
	        }
	        if !matched {
	                node.Taints = append(node.Taints, taint)
	        }
        }
	if _, err = client.Management.Node.Update(node, node); err != nil {
		err = fmt.Errorf("rancher2-client.NodeSetAnnotationsLabelsTaints() error: %s", err)
	}
	return
}

// NodeGetLabels get Rancher node labels
func (client *Rancher2Client) NodeGetLabels(nodeID string) (nodeLabels map[string]string, err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeGetLabels() error: %s", err)
	} else {
	        nodeLabels = node.Labels
	}
	return
}

// NodeUpdateLabels updates Rancher node labels
func (client *Rancher2Client) NodeUpdateLabels(nodeID string, oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeUpdateLabels().GetNodeByID() error: %s", err)
		return
	}
	for key := range oldLabels {
		delete(node.Labels, key)
	}
	for key, elem := range newLabels {
		node.Labels[key] = elem.(string)
	}
	if _, err = client.Management.Node.Update(node, node); err != nil {
		err = fmt.Errorf("rancher2-client.NodeUpdateLabels() error: %s", err)
	}
	return
}

// NodeGetTaints get Rancher node taints
func (client *Rancher2Client) NodeGetTaints(nodeID string) (taints []managementClient.Taint, err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeGetTaints() error: %s", err)
	} else {
	        taints = node.Taints
	}
	return
}

// NodeUpdateTaints updates Rancher node taints
func (client *Rancher2Client) NodeUpdateTaints(nodeID string, oldTaints []interface{}, newTaints []interface{}) (err error) {
	var node *managementClient.Node
	var taints []managementClient.Taint
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.NodeUpdateTaints().GetNodeByID() error: %s", err)
		return
	}
	for _, taint := range node.Taints {
	        matched := false
	        for _, oldTaint := range oldTaints {
	                if oldTaint.(map[string]interface{})["key"].(string) == taint.Key && oldTaint.(map[string]interface{})["value"].(string) == taint.Value && oldTaint.(map[string]interface{})["effect"].(string) == taint.Effect {
	                        matched = true
	                }
	        }
	        if !matched {
	                taints = append(taints, taint)
	        }
        }
	for _, newTaint := range newTaints {
	        matched := false
	        for _, taint := range taints {
	                if newTaint.(map[string]interface{})["key"].(string) == taint.Key && newTaint.(map[string]interface{})["value"].(string) == taint.Value && newTaint.(map[string]interface{})["effect"].(string) == taint.Effect {
	                        matched = true
	                }
	        }
	        if !matched {
	                taints = append(
	                        taints,
	                        managementClient.Taint{
	                                Key: newTaint.(map[string]interface{})["key"].(string),
	                                Value: newTaint.(map[string]interface{})["value"].(string),
	                                Effect: newTaint.(map[string]interface{})["effect"].(string),
	                        })
	        }
        }
        node.Taints = taints
	if _, err = client.Management.Node.Update(node, node); err != nil {
		err = fmt.Errorf("rancher2-client.NodeUpdateTaints() error: %s", err)
	}
	return
}
