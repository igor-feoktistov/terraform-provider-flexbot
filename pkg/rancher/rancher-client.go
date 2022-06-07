package rancher

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	rancher2ClientAPIVersion         = "/v3"
	rancher2ReadyAnswer              = "pong"
	rancher2RetriesWait              = 5
	rancher2RKEK8sSystemImageVersion = "2.3.0"
	maxHTTPRedirect                  = 5
)

// Client is Rancher client
type Client struct {
	Management *managementClient.Client
	CatalogV2  map[string]*clientbase.APIBaseClient
	Cluster    map[string]*clusterClient.Client
}

// Config is Rancher client config
type Config struct {
	TokenKey             string `json:"tokenKey"`
	URL                  string `json:"url"`
	CACerts              string `json:"cacert"`
	Insecure             bool   `json:"insecure"`
	Bootstrap            bool   `json:"bootstrap"`
	ClusterID            string `json:"clusterId"`
	ProjectID            string `json:"projectId"`
	Retries              int
	RancherVersion       string
	K8SDefaultVersion    string
	K8SSupportedVersions []string
	Sync                 sync.Mutex
	NodeDrainInput       *managementClient.NodeDrainInput
	Client               Client
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

// IsVersionLessThan verifies version
func IsVersionLessThan(ver1, ver2 string) (bool, error) {
	v1, err := version.NewVersion(ver1)
	if err != nil {
		return false, err
	}
	v2, err := version.NewVersion(ver2)
	if err != nil {
		return false, err
	}
	return v1.LessThan(v2), nil
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

// GetRancherVersion gets Rancher version
func (c *Config) GetRancherVersion() (string, error) {
	if len(c.RancherVersion) > 0 {
		return c.RancherVersion, nil
	}
	if c.Client.Management == nil {
		err := c.ManagementClient()
		if err != nil {
			return "", err
		}
	}
	version, err := c.Client.Management.Setting.ByID("server-version")
	if err != nil {
		return "", fmt.Errorf("[ERROR] Getting Rancher version: %s", err)
	}
	c.RancherVersion = version.Value
	return c.RancherVersion, nil
}

func (c *Config) isRancherReady() error {
	var err error
	var resp []byte
	url := RootURL(c.URL) + "/ping"
	for i := 0; i <= c.Retries; i++ {
		resp, err = DoGet(url, "", "", "", c.CACerts, c.Insecure)
		if err == nil && rancher2ReadyAnswer == string(resp) {
			return nil
		}
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	return fmt.Errorf("rancher is not ready: %v", err)
}

// IsRancherVersionLessThan compares Rancher version
func (c *Config) IsRancherVersionLessThan(ver string) (bool, error) {
	if len(ver) == 0 {
		return false, fmt.Errorf("[ERROR] version is nil")
	}
	_, err := c.GetRancherVersion()
	if err != nil {
		return false, fmt.Errorf("[ERROR] getting rancher server version")
	}
	return IsVersionLessThan(c.RancherVersion, ver)
}

// ManagementClient initializes Rancher Management Client
func (c *Config) ManagementClient() error {
	c.Sync.Lock()
	defer c.Sync.Unlock()
	if c.Client.Management != nil {
		return nil
	}
	err := c.isRancherReady()
	if err != nil {
		return err
	}
	options := c.CreateClientOpts()
	options.URL = options.URL + rancher2ClientAPIVersion
	mClient, err := managementClient.NewClient(options)
	if err != nil {
		return err
	}
	c.Client.Management = mClient
	return err
}

// CreateClientOpts creates client options
func (c *Config) CreateClientOpts() *clientbase.ClientOpts {
	c.NormalizeURL()
	options := &clientbase.ClientOpts{
		URL:      c.URL,
		TokenKey: c.TokenKey,
		CACerts:  c.CACerts,
		Insecure: c.Insecure,
	}
	return options
}

// NormalizeURL normalizes URL
func (c *Config) NormalizeURL() {
	c.URL = NormalizeURL(c.URL)
}

// GetNode gets Rancher node by cluster ID and node IP address
func (client *Client) GetNode(clusterID string, nodeIPAddr string) (nodeID string, err error) {
	var clusters *managementClient.ClusterCollection
	var nodes *managementClient.NodeCollection
	filters := map[string]interface{}{
		"id": clusterID,
	}
	clusters, err = client.Management.Cluster.List(NewListOpts(filters))
	if err == nil && len(clusters.Data) > 0 {
		filters := map[string]interface{}{
			"clusterId": clusterID,
			"ipAddress": nodeIPAddr,
		}
		nodes, err = client.Management.Node.List(NewListOpts(filters))
		if err == nil && len(nodes.Data) > 0 {
			nodeID = nodes.Data[0].ID
		}
	}
	if err != nil {
		err = fmt.Errorf("rancher.GetNode() error: %s", err)
	}
	return
}

// GetNodeRole gets Rancher node role
func (client *Client) GetNodeRole(nodeID string) (controlplane bool, etcd bool, worker bool, err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
		return
	}
	return node.ControlPlane, node.Etcd, node.Worker, nil
}

// ClusterWaitForState waits until cluster in specified state
func (client *Client) ClusterWaitForState(clusterID string, states string, timeout int) (err error) {
	var cluster *managementClient.Cluster
	var clusterLastState string
	var settleDown int
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if cluster, err = client.Management.Cluster.ByID(clusterID); err != nil {
			if IsNotFound(err) || IsForbidden(err) {
				err = fmt.Errorf("rancher.ClusterWaitForState(): cluster not found")
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
	err = fmt.Errorf("rancher.ClusterWaitForState(): wait for cluster state exceeded timeout=%d: expected states=%s, last state=%s", timeout, states, clusterLastState)
	return
}

// ClusterWaitForTransitioning waits until cluster enters transitioning state
func (client *Client) ClusterWaitForTransitioning(clusterID string, timeout int) (err error) {
	var cluster *managementClient.Cluster
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if cluster, err = client.Management.Cluster.ByID(clusterID); err != nil {
			if IsNotFound(err) || IsForbidden(err) {
				err = fmt.Errorf("rancher.ClusterWaitForTransitioning(): cluster not found")
			}
			return
		}
		if (cluster.State == "updating" || cluster.State == "upgrading") && (cluster.Transitioning == "yes" || cluster.Transitioning == "error") {
		        return
		}
		time.Sleep(1 * time.Second)
	}
	err = fmt.Errorf("rancher.ClusterWaitForTransitioning(): wait for cluster transitioning exceeded timeout=%d", timeout)
	return
}

// NodeGetState gets Rancher node state
func (client *Client) NodeGetState(nodeID string) (state string, err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
	} else {
		state = node.State
	}
	return
}

// NodeWaitForState waits until Rancher node in specified state
func (client *Client) NodeWaitForState(nodeID string, states string, timeout int) (err error) {
	var node *managementClient.Node
	var nodeLastState string
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
	for time.Now().Before(giveupTime) {
		if node, err = client.Management.Node.ByID(nodeID); err != nil {
			err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
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
	err = fmt.Errorf("rancher.NodeWaitForState(): wait for node state exceeded timeout=%d: expected states=%s, last state=%s", timeout, states, nodeLastState)
	return
}

// NodeCordon cordon Rancher node
func (client *Client) NodeCordon(nodeID string) (err error) {
	var node *managementClient.Node
	var ok bool
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
		return
	}
	_, ok = node.Actions["cordon"]
	if ok {
                err = client.Management.Node.ActionCordon(node)
	}
	return
}

// NodeCordonDrain cordon/drain Rancher node
func (client *Client) NodeCordonDrain(nodeID string, nodeDrainInput *managementClient.NodeDrainInput) (err error) {
	var node *managementClient.Node
	var ok bool
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
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
					var state string
					if state, err = client.NodeGetState(nodeID); err == nil {
						if !(state == "cordoned" || state == "drained") {
							err = fmt.Errorf("expected node state either \"cordoned\" or \"drained\"")
						}
					}
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("rancher.NodeCordonDrain() error: %s", err)
	}
	return
}

// NodeUncordon uncordon Rancher node
func (client *Client) NodeUncordon(nodeID string) (err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
		return
	}
	_, ok := node.Actions["uncordon"]
	if ok {
		err = client.Management.Node.ActionUncordon(node)
	}
	if err != nil {
		err = fmt.Errorf("rancher.NodeUncordon() error: %s", err)
	}
	return
}

// DeleteNode deletes Rancher node
func (client *Client) DeleteNode(nodeID string) (err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
		return
	}
	err = client.Management.Node.Delete(node)
	if err != nil {
		err = fmt.Errorf("rancher.DeleteNode() error: %s", err)
	}
	return
}

// NodeSetAnnotationsLabelsTaints sets Rancher node annotations, labels, and taints
func (client *Client) NodeSetAnnotationsLabelsTaints(nodeID string, annotations map[string]string, labels map[string]string, taints []managementClient.Taint) (err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
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
		err = fmt.Errorf("rancher.NodeSetAnnotations() error: %s", err)
	}
	return
}

// NodeUpdateLabels updates Rancher node labels
func (client *Client) NodeUpdateLabels(nodeID string, oldLabels map[string]interface{}, newLabels map[string]interface{}) (err error) {
	var node *managementClient.Node
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
		return
	}
	for key := range oldLabels {
		delete(node.Labels, key)
	}
	for key, elem := range newLabels {
		node.Labels[key] = elem.(string)
	}
	if _, err = client.Management.Node.Update(node, node); err != nil {
		err = fmt.Errorf("rancher.NodeSetLabels() error: %s", err)
	}
	return
}

// NodeUpdateTaints updates Rancher node taints
func (client *Client) NodeUpdateTaints(nodeID string, oldTaints []interface{}, newTaints []interface{}) (err error) {
	var node *managementClient.Node
	var taints []managementClient.Taint
	if node, err = client.Management.Node.ByID(nodeID); err != nil {
		err = fmt.Errorf("rancher.Node.ByID() error: %s", err)
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
		err = fmt.Errorf("rancher.NodeSetLabels() error: %s", err)
	}
	return
}
