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
        HostNameLabel = "kubernetes.io/hostname"
        NodeRoleLabelWorker = "node-role.kubernetes.io/worker"
        NodeRoleLabelControlplane = "node-role.kubernetes.io/controlplane"
        NodeRoleLabelEtcd = "node-role.kubernetes.io/etcd"
)

// RkeClient is RKE client
type RkeClient struct {
	Management *kubernetes.Clientset
}

// GetNode gets RKE node by node IP address
func (client *RkeClient) GetNode(nodeIpAddr string) (nodeName string, err error) {
        var nodes *v1.NodeList
        if nodes, err = client.Management.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{}); err != nil {
		err = fmt.Errorf("rke-client.GetNode() error: %s", err)
		return
        }
	for _, item := range nodes.Items {
	        var hostIpAddr net.IP
	        if hostIpAddr, err = node.GetNodeHostIP(&item); err != nil {
		        err = fmt.Errorf("rke-client.GetNode().node.GetNodeHostIP() error: %s", err)
		        return
                }
                if hostIpAddr.String() == nodeIpAddr {
                        nodeName = item.Name
		        break
                }
        }
	return
}

// GetNodeRole gets RKE node role
func (client *RkeClient) GetNodeRole(nodeName string) (controlplane bool, etcd bool, worker bool, err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.GetNodeRole(%s) error: %s", nodeName, err)
		return
	}
        if strings.ToLower(node.Labels[NodeRoleLabelWorker]) == "true" {
                worker = true
        }
        if strings.ToLower(node.Labels[NodeRoleLabelControlplane]) == "true" {
                controlplane = true
        }
        if strings.ToLower(node.Labels[NodeRoleLabelEtcd]) == "true" {
                etcd = true
        }
	return
}

// NodeCordon cordon RKE node
func (client *RkeClient) NodeCordon(nodeName string) (err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeCordon() error: %s", err)
		return
	}
	node.Spec.Unschedulable = true
	if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeCordon() error: %s", err)
	}
	return
}

// NodeCordonDrain cordon and drain RKE node
func (client *RkeClient) NodeCordonDrain(nodeName string, nodeDrainInput *config.NodeDrainInput) (err error) {
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
	drainTimeMax := time.Now().Add(time.Second * time.Duration(nodeDrainInput.Timeout))
        if err = drain.RunNodeDrain(drainer, nodeName); err != nil {
                // Do not fail drains which exceeded drain maximum time
		if time.Now().Before(drainTimeMax) {
		        err = fmt.Errorf("rke-client.NodeCordonDrain() error: %s", err)
		} else {
		        err = nil
		}
        }
        return

}

// NodeUncordon uncordon RKE node
func (client *RkeClient) NodeUncordon(nodeName string) (err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUncordon() error: %s", err)
		return
	}
	node.Spec.Unschedulable = false
	if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUncordon() error: %s", err)
	}
	return
}

// NodeSetAnnotationsLabelsTaints sets Rancher node annotations, labels, and taints
func (client *RkeClient) NodeSetAnnotationsLabelsTaints(nodeName string, annotations map[string]string, labels map[string]string, taints []v1.Taint) (err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeSetAnnotationsLabelsTaints() error: %s", err)
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
	        for _, nodeTaint := range node.Spec.Taints {
	                if taint.Key == nodeTaint.Key && taint.Value == nodeTaint.Value && taint.Effect == nodeTaint.Effect {
	                        matched = true
	                }
	        }
	        if !matched {
	                node.Spec.Taints = append(node.Spec.Taints, taint)
	        }
        }
	if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeSetAnnotationsLabelsTaints() error: %s", err)
	}
	return
}

// NodeGetLabels get RKE node labels
func (client *RkeClient) NodeGetLabels(nodeName string) (nodeLabels map[string]string, err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeGetLabels() error: %s", err)
	} else {
	        nodeLabels = node.Labels
	}
	return
}

// NodeUpdateLabels updates RKE node labels
func (client *RkeClient) NodeUpdateLabels(nodeName string, oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUpdateLabels() error: %s", err)
		return
	}
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
	if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUpdateLabels() error: %s", err)
	}
	return
}

// NodeGetTaints get RKE node taints
func (client *RkeClient) NodeGetTaints(nodeName string) (nodeTaints []v1.Taint, err error) {
        var node *v1.Node
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeGetTaints() error: %s", err)
	} else {
	        nodeTaints = node.Spec.Taints
        }
	return
}

// NodeUpdateTaints updates RKE node taints
func (client *RkeClient) NodeUpdateTaints(nodeName string, oldTaints []interface{}, newTaints []interface{}) (err error) {
        var node *v1.Node
	var taints []v1.Taint
	if node, err = client.Management.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUpdateTaints() error: %s", err)
		return
	}
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
	if _, err = client.Management.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
		err = fmt.Errorf("rke-client.NodeUpdateTaints() error: %s", err)
	}
        return
}
