package rancher

import (
        "context"
	"fmt"
	"net"
	"strings"
	"io/ioutil"
	"time"

        v1 "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubectl/pkg/drain"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	Rke2HostNameLabel = "kubernetes.io/hostname"
        Rke2NodeRoleLabelWorker = "node-role.kubernetes.io/worker"
        Rke2NodeRoleLabelControlplane = "node-role.kubernetes.io/control-plane"
        Rke2NodeRoleLabelEtcd = "node-role.kubernetes.io/etcd"
	rke2RetriesWait = 5
)

// Rke2Client is RKE2 client
type Rke2Client struct {
	RancherConfig *config.RancherConfig
	Management    *kubernetes.Clientset
}

// IsTransientError returns true in case of transient error
func (client *Rke2Client) IsTransientError(err error) (bool) {
	if err != nil {
		if strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
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
	}
	return false
}

// IsNotFoundError returns true in case of "not found" error
func (client *Rke2Client) IsNotFoundError(err error) (bool) {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") {
		return true
	}
	return false
}

// GetNode gets RKE2 node name by node IP address
func (client *Rke2Client) GetNodeName(nodeIpAddr string) (nodeName string, err error) {
        var nodes *v1.NodeList
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if nodes, err = client.Management.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{}); err == nil {
			for _, item := range nodes.Items {
	        		var hostIpAddr net.IP
	        		if hostIpAddr, err = node.GetNodeHostIP(&item); err != nil {
		        		err = fmt.Errorf("rke2-client.GetNodeName().node.GetNodeHostIP() error: %s", err)
		        		return
                		}
                		if hostIpAddr.String() == nodeIpAddr {
                        		nodeName = item.Name
		        		return
                		}
        		}
			err = fmt.Errorf("rke2-client.GetNodeName().node.GetNodeHostIP() error: node with IP address %s not found", nodeIpAddr)
		        return
        	}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.GetNodeName() error: %s", err)
	}
	return
}

// GetNodeByName gets RKE2 node by node name
func (client *Rke2Client) GetNodeByName(nodeName string) (node *v1.Node, err error) {
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
			return
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.GetNodeByName(%s) error: %s", nodeName, err)
	}
	return
}

// GetNodeRole gets RKE2 node role
func (client *Rke2Client) GetNodeRole(nodeName string) (controlplane bool, etcd bool, worker bool, err error) {
        var node *v1.Node
	if node, err = client.GetNodeByName(nodeName); err == nil {
		if strings.ToLower(node.Labels[Rke2NodeRoleLabelWorker]) == "true" {
			worker = true
		}
		if strings.ToLower(node.Labels[Rke2NodeRoleLabelControlplane]) == "true" {
			controlplane = true
		}
		if strings.ToLower(node.Labels[Rke2NodeRoleLabelEtcd]) == "true" {
			etcd = true
		}
	} else {
		err = fmt.Errorf("rke2-client.GetNodeRole(%s) error: %s", nodeName, err)
	}
	return
}

// IsNodeReady returns true if node ready
func (client *Rke2Client) IsNodeReady(nodeName string) (ready bool, err error) {
        var nodeObj *v1.Node
	if nodeObj, err = client.GetNodeByName(nodeName); err == nil {
		ready = node.IsNodeReady(nodeObj)
	} else {
		err = fmt.Errorf("rke2-client.IsNodeReady(%s) error: %s", nodeName, err)
	}
	return
}

// NodeDelete deletes RKE2 node
func (client *Rke2Client) NodeDelete(nodeName string) (err error) {
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if _, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
			if err = client.Management.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{}); err == nil {
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
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rk2-client.NodeDelete() error: %s", err)
	}
	return
}

// NodeCordon cordon RKE2 node
func (client *Rke2Client) NodeCordon(nodeName string) (err error) {
        var node *v1.Node
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
			node.Spec.Unschedulable = true
			if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
				break
			}
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeCordon() error: %s", err)
	}
	return
}

// NodeCordonDrain cordon and drain RKE2 node
func (client *Rke2Client) NodeCordonDrain(nodeName string, nodeDrainInput *config.NodeDrainInput) (err error) {
        if err = client.NodeCordon(nodeName); err != nil {
                return
        }
        drainer := &drain.Helper{
                Ctx:                 context.TODO(),
                Client:              client.Management,
                Force:               nodeDrainInput.Force,
                GracePeriodSeconds:  int(nodeDrainInput.GracePeriod),
                IgnoreAllDaemonSets: *nodeDrainInput.IgnoreDaemonSets,
                Out:                 ioutil.Discard,
                ErrOut:              ioutil.Discard,
                DeleteEmptyDirData:  nodeDrainInput.DeleteLocalData,
                Timeout:             time.Second * time.Duration(nodeDrainInput.Timeout),
        }
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
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeCordonDrain() error: %s", err)
        }
        return

}

// NodeUncordon uncordon RKE2 node
func (client *Rke2Client) NodeUncordon(nodeName string) (err error) {
        var node *v1.Node
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
			node.Spec.Unschedulable = false
			if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
				break
			}
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeUncordon() error: %s", err)
	}
	return
}

// NodeSetAnnotationsLabelsTaints sets Rancher node annotations, labels, and taints
func (client *Rke2Client) NodeSetAnnotationsLabelsTaints(nodeName string, annotations map[string]string, labels map[string]string, taints []v1.Taint) (err error) {
        var node *v1.Node
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
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
			if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
				break
			}
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeSetAnnotationsLabelsTaints() error: %s", err)
	}
	return
}

// NodeGetLabels get RKE2 node labels
func (client *Rke2Client) NodeGetLabels(nodeName string) (nodeLabels map[string]string, err error) {
        var node *v1.Node
	if node, err = client.GetNodeByName(nodeName); err == nil {
	        nodeLabels = node.Labels
	} else {
		err = fmt.Errorf("rke2-client.NodeGetLabels() error: %s", err)
	}
	return
}

// NodeUpdateLabels updates RKE2 node labels
func (client *Rke2Client) NodeUpdateLabels(nodeName string, oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
        var node *v1.Node
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
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
			if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
				break
			}
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeUpdateLabels() error: %s", err)
	}
	return
}

// NodeGetTaints get RKE2 node taints
func (client *Rke2Client) NodeGetTaints(nodeName string) (nodeTaints []v1.Taint, err error) {
        var node *v1.Node
	if node, err = client.GetNodeByName(nodeName); err == nil {
		nodeTaints = node.Spec.Taints
	} else {
		err = fmt.Errorf("rke2-client.NodeGetTaints() error: %s", err)
	}
	return
}

// NodeUpdateTaints updates RKE2 node taints
func (client *Rke2Client) NodeUpdateTaints(nodeName string, oldTaints []interface{}, newTaints []interface{}) (err error) {
        var node *v1.Node
	for retry := 0; retry < client.RancherConfig.Retries; retry++ {
		var taints []v1.Taint
		if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err == nil {
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
			if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err == nil {
				break
			}
		}
		if !client.IsTransientError(err) {
			break
		}
		time.Sleep(rke2RetriesWait * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("rke2-client.NodeUpdateTaints() error: %s", err)
	}
        return
}
