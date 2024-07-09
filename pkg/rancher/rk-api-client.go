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
	//"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

const (
	rkApiRetriesWait = 5
)

// RkApiClient is RK-API client
type RkApiClient struct {
	LocalClusterClient      dynamic.Interface
	DownstreamClusterClient *kubernetes.Clientset
}

func getMapValue(m interface{}, key string) interface{} {
        v, ok := m.(map[string]interface{})[key]
        if !ok {
                return nil
        }
        return v
}

func (client *RkApiClient) GetMachine(clusterName string, nodeIp string) (machine *unstructured.Unstructured, err error) {
	var machineList *unstructured.UnstructuredList
	machineClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "machines"})
	opt := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("cluster.x-k8s.io/cluster-name=%s", clusterName),
	}
	if machineList, err = machineClient.List(context.Background(), opt); err == nil {
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

func (client *RkApiClient) GetMachineState(machineName string) (state string, err error) {
	var machineList *unstructured.UnstructuredList
	machineClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "machines"})
	opt := metav1.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector("metadata.name", machineName).String(),
	}
	if machineList, err = machineClient.List(context.Background(), opt); err == nil {
		for _, item := range machineList.Items {
			if name := getMapValue(item.UnstructuredContent()["metadata"], "name"); name != nil && name == machineName {
				phase := getMapValue(item.UnstructuredContent()["status"], "phase")
				infrastructureReady := getMapValue(item.UnstructuredContent()["status"], "infrastructureReady")
				if phase != nil && phase.(string) == "Running" && infrastructureReady != nil && infrastructureReady.(bool) {
					state = "active"
					if conditions := getMapValue(item.UnstructuredContent()["status"], "conditions"); conditions != nil {
						for _, condition := range conditions.([]interface{}) {
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "Ready" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "InfrastructureReady" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "PlanApplied" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "Reconciled" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
						}
					}
				} else {
					state = "notReady"
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

func (client *RkApiClient) GetClusterState(clusterName string) (state string, err error) {
	var clusterList *unstructured.UnstructuredList
	clusterClient := client.LocalClusterClient.Resource(schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "clusters"})
	opt := metav1.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector("metadata.name", clusterName).String(),
	}
	if clusterList, err = clusterClient.List(context.Background(), opt); err == nil {
		for _, item := range clusterList.Items {
			if name := getMapValue(item.UnstructuredContent()["metadata"], "name"); name != nil && name == clusterName {
				phase := getMapValue(item.UnstructuredContent()["status"], "phase")
				infrastructureReady := getMapValue(item.UnstructuredContent()["status"], "infrastructureReady")
				controlPlaneReady := getMapValue(item.UnstructuredContent()["status"], "controlPlaneReady")
				if phase != nil && phase.(string) == "Provisioned" && infrastructureReady != nil && infrastructureReady.(bool) && controlPlaneReady != nil && controlPlaneReady.(bool) {
					state = "active"
					if conditions := getMapValue(item.UnstructuredContent()["status"], "conditions"); conditions != nil {
						for _, condition := range conditions.([]interface{}) {
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "ControlPlaneReady" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
							if conditionType, ok := condition.(map[string]interface{})["type"]; ok && conditionType.(string) == "InfrastructureReady" {
								if conditionStatus, ok := condition.(map[string]interface{})["status"]; ok && conditionStatus.(string) != "True" {
									state = "inTransition"
								}
							}
						}
					}
				} else {
					state = "notReady"
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

// NodeCordon cordon Kubernetes node
func (client *RkApiClient) NodeCordon(nodeName string) (err error) {
        var node *v1.Node
	if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rk-api-client.NodeCordon() error: %s", err)
		return
	}
	node.Spec.Unschedulable = true
	if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rk-api-client.NodeCordon() error: %s", err)
	}
	return
}

// NodeUncordon uncordon Kubernetes node
func (client *RkApiClient) NodeUncordon(nodeName string) (err error) {
        var node *v1.Node
	if node, err = client.DownstreamClusterClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rk-api-client.NodeUncordon() error: %s", err)
		return
	}
	node.Spec.Unschedulable = false
	if _, err = client.DownstreamClusterClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rk-api-client.NodeUncordon() error: %s", err)
	}
	return
}

// NodeCordonDrain cordon and drain Kubernetes node
func (client *RkApiClient) NodeCordonDrain(nodeName string, nodeDrainInput *rancherManagementClient.NodeDrainInput) (err error) {
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
	drainTimeMax := time.Now().Add(time.Second * time.Duration(nodeDrainInput.Timeout))
        if err = drain.RunNodeDrain(drainer, nodeName); err != nil {
                // Do not fail drains which exceeded drain maximum time
		if time.Now().Before(drainTimeMax) {
		        err = fmt.Errorf("rk-api-client.NodeCordonDrain() error: %s", err)
		} else {
		        err = nil
		}
        }
        return
}
