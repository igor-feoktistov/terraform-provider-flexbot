package rancher

import (
	"fmt"
	"strings"
	"context"
	"time"
	"io/ioutil"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	rkApiRetriesWait = 5
	rkApiStabilizeWait = 3
	rkApiStabilizeMax = 10
)

// RkApiClient is RK-API client
type RkApiClient struct {
	RancherConfig           *config.RancherConfig
	LocalClusterClient      dynamic.Interface
	DownstreamClusterClient *kubernetes.Clientset
}

// Retry Rancher server URL probes number of "Retries" attempts or until ready
func (client *RkApiClient) IsRancherReady() (err error) {
	var resp []byte
	if resp, err = DoGet(NormalizeURL(client.RancherConfig.URL) + rancherReadyRequest, "", "", "", string(client.RancherConfig.ServerCAData), client.RancherConfig.Insecure); err == nil && string(resp) == rancherReadyResponse {
		return
	}
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
	        for stabilize := 0; stabilize < rkApiStabilizeMax; stabilize++ {
			resp, err = DoGet(NormalizeURL(client.RancherConfig.URL) + rancherReadyRequest, "", "", "", string(client.RancherConfig.ServerCAData), client.RancherConfig.Insecure)
			if err == nil && rancherReadyResponse == string(resp) {
				time.Sleep(rkApiStabilizeWait * time.Second)
			} else {
				break
			}
		}
		if err == nil && rancherReadyResponse == string(resp) {
			return
                }
		time.Sleep(rkApiRetriesWait * time.Second)
	}
	return fmt.Errorf("rk-api-client.IsRancherReady(): rancher is not ready after %d attempts in %d seconds, last error: %s", client.RancherConfig.Retries, rkApiRetriesWait * client.RancherConfig.Retries, err)
}

// Resilient to transient errors GetMachineList
func (client *RkApiClient) GetMachineList(opt metav1.ListOptions) (machineList *unstructured.UnstructuredList, err error) {
	machineClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: CAPI_Group, Version: CAPI_Version, Resource: CAPI_MachineResource})
	if err = client.IsRancherReady(); err == nil {
		machineList, err = machineClient.List(context.Background(), opt)
	}
	return
}

// GetMachineByNodeIp search machine by cluster name and node IP
func (client *RkApiClient) GetMachineByNodeIp(clusterName string, nodeIp string) (machine *unstructured.Unstructured, err error) {
	var machineList *unstructured.UnstructuredList
	opt := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s/cluster-name=%s", CAPI_Group, clusterName),
	}
	if machineList, err = client.GetMachineList(opt); err == nil {
		for _, item := range machineList.Items {
			if addresses := getMapValue(item.UnstructuredContent()["status"], "addresses"); addresses != nil {
				for _, address := range addresses.([]interface{}) {
					if ip := getMapValue(address, "address"); ip != nil && ip.(string) == nodeIp {
						machine = &item
						return
					}
				}
			}
		}
		err = fmt.Errorf("rk-api-client.GetMachine() failure: no machine found in cluster \"%s\" with node IP address \"%s\"", clusterName, nodeIp)
	} else {
		err = fmt.Errorf("rk-api-client.GetMachine() failure: %s", err)
	}
        return
}

// GetMachineByName get machine by machine name
func (client *RkApiClient) GetMachineByName(machineName string) (machine *unstructured.Unstructured, err error) {
	var machineList *unstructured.UnstructuredList
	opt := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", machineName).String(),
	}
	if machineList, err = client.GetMachineList(opt); err == nil {
		for _, item := range machineList.Items {
			if name := getMapValue(item.UnstructuredContent()["metadata"], "name"); name != nil && name == machineName {
				machine = &item
				return
			}
		}
		err = fmt.Errorf("rk-api-client.GetMachine() failure: no machine found with name \"%s\"", machineName)
	} else {
		err = fmt.Errorf("rk-api-client.GetMachine() failure: %s", err)
	}
        return
}

// GetMachineCondition get machine condition details
func (client *RkApiClient) GetMachineCondition(machine *unstructured.Unstructured) (string) {
	if conditions := getMapValue(machine.UnstructuredContent()["status"], "conditions"); conditions != nil {
		for _, condition := range conditions.([]interface{}) {
			if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
				if conditionType, ok := condition.(map[string]interface{})["type"]; ok {
					if conditionReason, ok := condition.(map[string]interface{})["reason"]; ok {
						if conditionMessage, ok := condition.(map[string]interface{})["message"]; ok {
							return fmt.Sprintf("type=%s, reason=%s, message=%s", conditionType.(string), conditionReason.(string), conditionMessage.(string))
						}
					}
				}
			}
		}
	}
        return "unknown"
}

// GetMachineState returns machine state: "active", "inTransition", or "notReady"
func (client *RkApiClient) GetMachineState(machineName string) (state string, err error) {
	var machineList *unstructured.UnstructuredList
	opt := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", machineName).String(),
	}
	if machineList, err = client.GetMachineList(opt); err == nil {
		for _, item := range machineList.Items {
			if name := getMapValue(item.UnstructuredContent()["metadata"], "name"); name != nil && name == machineName {
				phase := getMapValue(item.UnstructuredContent()["status"], "phase")
				infrastructureReady := getMapValue(item.UnstructuredContent()["status"], "infrastructureReady")
				state = "notReady"
				if phase != nil && phase.(string) == "Running" && infrastructureReady != nil && infrastructureReady.(bool) {
					if conditions := getMapValue(item.UnstructuredContent()["status"], "conditions"); conditions != nil {
						state = "active"
						for _, condition := range conditions.([]interface{}) {
							if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
								state = "inTransition"
							}
						}
					}
				}
				return
			}
		}
		err = fmt.Errorf("rk-api-client.GetMachine() failure: no machine found with name \"%s\"", machineName)
	} else {
		err = fmt.Errorf("rk-api-client.GetMachine() failure: %s", err)
	}
        return
}

// DeleteMachine deletes node machine
func (client *RkApiClient) DeleteMachine(machineName string) (err error) {
	var machine *unstructured.Unstructured
	if machine, err = client.GetMachineByName(machineName); err != nil {
		if client.IsMachineNotFound(err) {
			err = nil
		}
		return
	}
	if machine != nil {
		u := machine.UnstructuredContent()
		machineName := GetMapString(u["metadata"], "name")
		machineNamespace := GetMapString(u["metadata"], "namespace")
		if len(machineName) > 0 && len(machineNamespace) > 0 {
			machineClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: CAPI_Group, Version: CAPI_Version, Resource: CAPI_MachineResource})
			machineClient.Namespace(machineNamespace).Delete(context.Background(), machineName, metav1.DeleteOptions{})
		}
	}
	return
}

// IsMachineNotFound returns true if error signifies "no machine found"
func (client *RkApiClient) IsMachineNotFound(err error) (bool) {
	if strings.HasPrefix(err.Error(), "rk-api-client.GetMachine() failure: no machine found") {
		return true
	}
	return false
}

// MachineWaitForState waits until Rancher machine in specified state
func (client *RkApiClient) MachineWaitForState(machineName string, state string, timeout int) (err error) {
	var machineState, machineLastState string
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if machineState, err = client.GetMachineState(machineName); err != nil {
			err = fmt.Errorf("rk-api-client.MachineWaitForState().GetMachineState() error: %s", err)
			return
		}
		if machineState == state {
			return
		}
		machineLastState = machineState
		time.Sleep(rkApiRetriesWait * time.Second)
	}
	err = fmt.Errorf("rk-api-client.MachineWaitForState(): wait for machine state exceeded timeout=%d: expected states=%s, last state=%s", timeout, state, machineLastState)
	return
}

// Resilient to transient errors GetClusterList
func (client *RkApiClient) GetClusterList(opt metav1.ListOptions) (clusterList *unstructured.UnstructuredList, err error) {
	clusterClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: CAPI_Group, Version: CAPI_Version, Resource: CAPI_ClusterResource})
	if err = client.IsRancherReady(); err == nil {
		clusterList, err = clusterClient.List(context.Background(), opt)
	}
	return
}

// GetClusterState resturns cluster state: "active", "inTransition", or "notReady"
func (client *RkApiClient) GetClusterState(clusterName string) (state string, err error) {
	var clusterList *unstructured.UnstructuredList
	opt := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", clusterName).String(),
	}
	if clusterList, err = client.GetClusterList(opt); err == nil {
		for _, item := range clusterList.Items {
			if name := getMapValue(item.UnstructuredContent()["metadata"], "name"); name != nil && name == clusterName {
				phase := getMapValue(item.UnstructuredContent()["status"], "phase")
				infrastructureReady := getMapValue(item.UnstructuredContent()["status"], "infrastructureReady")
				controlPlaneReady := getMapValue(item.UnstructuredContent()["status"], "controlPlaneReady")
				state = "notReady"
				if phase != nil && phase.(string) == "Provisioned" && infrastructureReady != nil && infrastructureReady.(bool) && controlPlaneReady != nil && controlPlaneReady.(bool) {
					if conditions := getMapValue(item.UnstructuredContent()["status"], "conditions"); conditions != nil {
						state = "active"
						for _, condition := range conditions.([]interface{}) {
							if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
								state = "inTransition"
							}
						}
					}
				}
				return
			}
		}
		err = fmt.Errorf("rk-api-client.List() failure: no cluster with name \"%s\" found", clusterName)
	} else {
		err = fmt.Errorf("rk-api-client.List() failure: %s", err)
	}
        return
}

// ClusterWaitForState waits until cluster is in specified state
func (client *RkApiClient) ClusterWaitForState(clusterName string, state string, timeout int) (err error) {
	var clusterState, clusterLastState string
	var settleDown int
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if clusterState, err = client.GetClusterState(clusterName); err != nil {
			return
		}
		if clusterLastState != clusterState {
	                settleDown = 10
		}
		if clusterState == state {
			if settleDown == 0 {
				return
			} else {
				settleDown--
			}
		}
		clusterLastState = clusterState
		time.Sleep(rkApiRetriesWait * time.Second)
	}
	if clusterLastState != state {
		err = fmt.Errorf("rk-api-client.ClusterWaitForState(): wait for cluster state exceeded timeout=%d: expected state=%s, last state=%s", timeout, state, clusterLastState)
	}
	return
}

// ClusterWaitForTransitioning waits until cluster enters transitioning state
func (client *RkApiClient) ClusterWaitForTransitioning(clusterName string, timeout int) (err error) {
	var clusterState string
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if clusterState, err = client.GetClusterState(clusterName); err != nil {
			return
		}
		if clusterState == "inTransition" {
		        return
		}
		time.Sleep(rkApiRetriesWait * time.Second)
	}
	err = fmt.Errorf("rk-api-client.ClusterWaitForTransitioning(): wait for cluster transitioning exceeded timeout=%d", timeout)
	return
}

// IsTransientError returns true in case of transient error
func (client *RkApiClient) IsTransientError(err error) (bool) {
	if strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
		return true
	}
	if strings.Contains(err.Error(), "tunnel disconnect") {
		return true
	}
	if strings.Contains(err.Error(), "connection timed out") {
		return true
	}
	if strings.Contains(err.Error(), "i/o timeout") {
		return true
	}
	if strings.Contains(err.Error(), "handshake timeout") {
		return true
	}
	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}
	if strings.Contains(err.Error(), "context deadline exceeded") {
		return true
	}
	if strings.Contains(err.Error(), "error trying to reach service") {
		return true
	}
	return false
}

// IsNotFoundError returns true in case of "not found" error
func (client *RkApiClient) IsNotFoundError(err error) (bool) {
	if strings.Contains(err.Error(), "not found") {
		return true
	}
	return false
}

// NodeCordon cordon Kubernetes node
func (client *RkApiClient) NodeCordon(nodeName string) (err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				node.Spec.Unschedulable = true
				if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
					break
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeCordon() error: %s", err)
	}
	return
}

// NodeUncordon uncordon Kubernetes node
func (client *RkApiClient) NodeUncordon(nodeName string) (err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				node.Spec.Unschedulable = false
				if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
					break
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeUncordon() error: %s", err)
	}
	return
}

// NodeCordonDrain cordon and drain Kubernetes node
func (client *RkApiClient) NodeCordonDrain(nodeName string, nodeDrainInput *config.NodeDrainInput) (err error) {
        if err = client.NodeCordon(nodeName); err != nil {
                return
        }
        drainer := &drain.Helper{
                Ctx:                 context.TODO(),
                Client:              client.DownstreamClusterClient,
                Force:               nodeDrainInput.Force,
                GracePeriodSeconds:  int(nodeDrainInput.GracePeriod),
                IgnoreAllDaemonSets: *nodeDrainInput.IgnoreDaemonSets,
                Out:                 ioutil.Discard,
                ErrOut:              ioutil.Discard,
                DeleteEmptyDirData:  nodeDrainInput.DeleteLocalData,
                Timeout:             time.Second * time.Duration(nodeDrainInput.Timeout),
        }
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			drainTimeMax := time.Now().Add(time.Second * time.Duration(nodeDrainInput.Timeout))
			if err = drain.RunNodeDrain(drainer, nodeName); err == nil {
				break
			}
                	// Do not fail drains which exceeded drain maximum time
			if time.Now().After(drainTimeMax) {
				err = nil
				break
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
        }
        if err != nil {
		err = fmt.Errorf("rk-api-client.NodeCordonDrain() error: %s", err)
        }
        return
}

// NodeSetAnnotationsLabelsTaints sets Rancher node annotations, labels, and taints
func (client *RkApiClient) NodeSetAnnotationsLabelsTaints(nodeName string, annotations map[string]string, labels map[string]string, taints []v1.Taint) (err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				for key, elem := range annotations {
					node.Annotations[key] = elem
				}
				for key, elem := range labels {
					node.Labels[key] = elem
				}
				for _, taint := range taints {
	        			matched := false
	        			for _, nodeTaint := range node.Spec.Taints {
	                			if taint.Key == nodeTaint.Key && taint.Value == nodeTaint.Value && taint.Effect == nodeTaint.Effect {
	                        			matched = true
	                			}
	        			}
	        			if !matched {
	                			node.Spec.Taints = append(node.Spec.Taints, taint)
	        			}
        			}
				if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
					break
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeSetAnnotationsLabelsTaints() error: %s", err)
	}
	return
}

// NodeGetLabels get node labels
func (client *RkApiClient) NodeGetLabels(nodeName string) (nodeLabels map[string]string, err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				nodeLabels = node.Labels
				break
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeGetLabels() error: %s", err)
	}
	return
}

// NodeUpdateLabels updates node labels
func (client *RkApiClient) NodeUpdateLabels(nodeName string, oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				if node.Labels == nil {
					node.Labels = map[string]string{}
				}
				for key := range oldLabels {
					if _, ok := node.Labels[key]; ok {
						delete(node.Labels, key)
					}
				}
				for key, value := range newLabels {
					node.Labels[key] = value.(string)
				}
				if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
					break
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeUpdateLabels() error: %s", err)
	}
	return
}

// NodeGetTaints get node taints
func (client *RkApiClient) NodeGetTaints(nodeName string) (nodeTaints []v1.Taint, err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				nodeTaints = node.Spec.Taints
				break
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeGetTaints() error: %s", err)
	}
	return
}

// NodeUpdateTaints updates node taints
func (client *RkApiClient) NodeUpdateTaints(nodeName string, oldTaints []interface{}, newTaints []interface{}) (err error) {
        var node *v1.Node
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			var taints []v1.Taint
			if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				for _, taint := range node.Spec.Taints {
	        			matched := false
	        			for _, oldTaint := range oldTaints {
	                			if oldTaint.(map[string]interface{})["key"].(string) == taint.Key && oldTaint.(map[string]interface{})["value"].(string) == taint.Value && oldTaint.(map[string]interface{})["effect"].(string) == string(taint.Effect) {
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
						if newTaint.(map[string]interface{})["key"].(string) == taint.Key && newTaint.(map[string]interface{})["value"].(string) == taint.Value && newTaint.(map[string]interface{})["effect"].(string) == string(taint.Effect) {
							matched = true
						}
					}
					if !matched {
						taints = append(
							taints,
							v1.Taint{
								Key: newTaint.(map[string]interface{})["key"].(string),
								Value: newTaint.(map[string]interface{})["value"].(string),
								Effect: v1.TaintEffect(newTaint.(map[string]interface{})["effect"].(string)),
							})
					}
				}
				node.Spec.Taints = taints
				if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
					break
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeUpdateTaints() error: %s", err)
	}
        return
}

// NodeDelete deletes node
func (client *RkApiClient) NodeDelete(nodeName string) (err error) {
	if err = client.IsRancherReady(); err == nil {
		for retry := 0; retry < client.RancherConfig.Retries; retry++ {
			if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
				if err = client.DownstreamClusterClient.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{}); err == nil {
					return
				}
			} else {
				if client.IsNotFoundError(err) {
					err = nil
					return
				}
			}
			if !client.IsTransientError(err) {
				break
			}
			time.Sleep(rkApiRetriesWait * time.Second)
		}
	}
	if err != nil {
		err = fmt.Errorf("rk-api-client.NodeDelete() error: %s", err)
	}
	return
}
