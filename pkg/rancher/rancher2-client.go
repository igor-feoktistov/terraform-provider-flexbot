package rancher

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	rancher2ClientAPIVersion = "/v3"
	rancher2ReadyAnswer = "pong"
	rancher2RetriesWait = 5
	rancher2StabilizeWait = 3
	rancher2StabilizeMax = 10
	maxHTTPRedirect = 5
)

// Rancher2Client is rancher2 client
type Rancher2Client struct {
	Management *managementClient.Client
	Retries int
}

// Rancher2Config is rancher2 client configuration
type Rancher2Config struct {
        config.RancherConfig
	Client Rancher2Client
}

// NormalizeURL normalizes URL
func NormalizeURL(input string) string {
	if input == "" {
		return ""
	}
	u, err := url.Parse(input)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	u.Path = ""
	return u.String()
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

// DoGet is core HTTP get routine
func DoGet(url, username, password, token, cacert string, insecure bool) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("doing get: URL is nil")
	}
	client := &http.Client{
		Timeout: time.Duration(60 * time.Second),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxHTTPRedirect {
				return fmt.Errorf("stopped after %d redirects", maxHTTPRedirect)
			}
			if len(token) > 0 {
				req.Header.Add("Authorization", "Bearer "+token)
			} else if len(username) > 0 && len(password) > 0 {
				s := username + ":" + password
				req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s)))
			}
			return nil
		},
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		Proxy:           http.ProxyFromEnvironment,
	}
	if cacert != "" {
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		rootCAs.AppendCertsFromPEM([]byte(cacert))
		transport.TLSClientConfig.RootCAs = rootCAs
	}
	client.Transport = transport
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("doing get: %v", err)
	}
	if len(token) > 0 {
		req.Header.Add("Authorization", "Bearer "+token)
	} else if len(username) > 0 && len(password) > 0 {
		s := username + ":" + password
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing get: %v", err)
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
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
		resp, err = DoGet(RootURL(c.URL) + "/ping", "", "", "", string(c.ServerCAData), c.Insecure)
		if err == nil && rancher2ReadyAnswer == string(resp) {
			break
		}
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	if err != nil {
	        err = fmt.Errorf("Rancher is not ready after %d attempts in %d seconds, last error: %s", c.Retries, rancher2RetriesWait * c.Retries, err)
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
		return
	}
	c.Client.Retries = c.Retries
	return
}

// Retry Rancher API probe number of "Retries" attempts or until ready
func (client *Rancher2Client) isRancherReady() (err error) {
	var resp []byte
	url := RootURL(client.Management.Opts.URL) + "/ping"
	for retry := 0; retry < client.Retries; retry++ {
	        for stabilize := 0; stabilize < rancher2StabilizeMax; stabilize++ {
		        resp, err = DoGet(url, "", "", "", client.Management.Opts.CACerts, client.Management.Opts.Insecure)
		        if err == nil && rancher2ReadyAnswer == string(resp) {
		                time.Sleep(rancher2StabilizeWait * time.Second)
		        } else {
		                break
		        }
		}
		if err == nil && rancher2ReadyAnswer == string(resp) {
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
func (client *Rancher2Client) GetNodeByAddr(clusterID string, nodeIPAddr string) (nodeID string, err error) {
	var clusters *managementClient.ClusterCollection
	var nodes *managementClient.NodeCollection
	filters := map[string]interface{}{
		"id": clusterID,
	}
	clusters, err = client.GetClusterList(NewListOpts(filters))
	if err == nil && len(clusters.Data) > 0 {
		filters := map[string]interface{}{
			"clusterId": clusterID,
			"ipAddress": nodeIPAddr,
		}
		nodes, err = client.GetNodeList(NewListOpts(filters))
		if err == nil && len(nodes.Data) > 0 {
			nodeID = nodes.Data[0].ID
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
		time.Sleep(1 * time.Second)
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
		time.Sleep(1 * time.Second)
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
		time.Sleep(1 * time.Second)
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

// DeleteNode deletes Rancher node
func (client *Rancher2Client) DeleteNode(nodeID string) (err error) {
	var node *managementClient.Node
	if node, err = client.GetNodeByID(nodeID); err != nil {
		err = fmt.Errorf("rancher2-client.DeleteNode().GetNodeByID() error: %s", err)
		return
	}
	if err = client.Management.Node.Delete(node); err != nil {
		err = fmt.Errorf("rancher2-client.DeleteNode() error: %s", err)
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
