package rancher

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	DO_RETRY_ATTEMPTS               = 10
        DO_RETRY_TIMEOUT                = 15
        MaintainanceStatusAnnotationKey = "harvesterhci.io/maintain-status"
        HarvesterNodeApiURI             = "/v1/harvester/nodes/"
	harvesterRetriesWait            = 5
	harvesterStabilizeWait          = 3
	harvesterStabilizeMax           = 10
)

type HarvesterClient struct {
	client          *http.Client
	BaseURL         *url.URL
	UserAgent       string
	options		*HarvesterClientOptions
	ResponseTimeout time.Duration
	Retries         int
}

type HarvesterClientOptions struct {
	BasicAuthToken    string
	SSLVerify         bool
	Debug             bool
	Timeout           time.Duration
}

type HarvesterErrorResponse struct {
	Message string `json:"message"`
	Code string    `json:"code"`
	Status int     `json:"status"`
	Type string    `json:"type"`
}

type HarvesterResponse struct {
	ErrorResponse HarvesterErrorResponse
	HttpResponse *http.Response
}

func NewHarvesterClient(endpoint string, options *HarvesterClientOptions) *HarvesterClient {
	if options == nil {
		options = &HarvesterClientOptions{
			SSLVerify: true,
			Debug:     false,
			Timeout:   60 * time.Second,
		}
	}
	httpClient := &http.Client {
		Timeout: options.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !options.SSLVerify,
			},
		},
	}
	if !strings.HasSuffix(endpoint, "/") {
		endpoint = endpoint + "/"
	}
	baseURL, _ := url.Parse(endpoint)
	c := &HarvesterClient{
		client:          httpClient,
		BaseURL:         baseURL,
		UserAgent:       "terraform-provider-flexbot",
		options:         options,
		ResponseTimeout: options.Timeout,
	}
	return c
}

func (c *HarvesterClient) NewRequest(method string, apiPath string, parameters []string, body interface{}) (req *http.Request, err error) {
	var payload io.Reader
	var extendedPath string
	if len(parameters) > 0 {
		extendedPath = fmt.Sprintf("%s?%s", apiPath, strings.Join(parameters, "&"))
	} else {
		extendedPath = apiPath
	}
	u, _ := c.BaseURL.Parse(extendedPath)
	if body != nil {
		buf, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			return nil, err
		}
		if c.options.Debug {
			log.Printf("[DEBUG] request JSON:\n%v\n\n", string(buf))
		}
		payload = bytes.NewBuffer(buf)
	}
	req, err = http.NewRequest(method, u.String(), payload)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.options.BasicAuthToken != "" {
		req.Header.Set("Authorization", "Basic " + base64.StdEncoding.EncodeToString(([]byte(c.options.BasicAuthToken))))
	}
	if c.options.Debug {
		dump, _ := httputil.DumpRequestOut(req, true)
		log.Printf("[DEBUG] request dump:\n%q\n\n", dump)
	}
	return
}

func (c *HarvesterClient) Do(req *http.Request, v interface{}) (resp *HarvesterResponse, err error) {
	ctx, cncl := context.WithTimeout(context.Background(), c.ResponseTimeout)
	defer cncl()
	for i := 0; i < DO_RETRY_ATTEMPTS; i++ {
		if resp, err = c.checkResp(c.client.Do(req.WithContext(ctx))); err == nil {
			break
		}
		if !(resp.HttpResponse.StatusCode == 429 || resp.HttpResponse.StatusCode == 502 || resp.HttpResponse.StatusCode == 503) {
			return
		}
		time.Sleep(time.Duration(DO_RETRY_TIMEOUT * (i + 1)) * time.Second)
	}
	if err != nil {
		return
	}
	var b []byte
	b, err = ioutil.ReadAll(resp.HttpResponse.Body)
	if err != nil {
		return
	}
	resp.HttpResponse.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	if c.options.Debug {
		log.Printf("[DEBUG] response JSON:\n%v\n\n", string(b))
	}
	if v != nil {
		defer resp.HttpResponse.Body.Close()
		err = json.NewDecoder(resp.HttpResponse.Body).Decode(v)
	}
	return
}

func (c *HarvesterClient) checkResp(resp *http.Response, err error) (*HarvesterResponse, error) {
	if err != nil {
		return &HarvesterResponse{HttpResponse: resp}, err
	}
	switch resp.StatusCode {
	case 200, 201, 202, 204, 205, 206:
		return &HarvesterResponse{HttpResponse: resp}, err
	default:
		restResp, httpErr := c.newHTTPError(resp)
		return restResp, httpErr
	}
}

func (c *HarvesterClient) newHTTPError(resp *http.Response) (restResp *HarvesterResponse, err error) {
	errResponse := HarvesterErrorResponse{}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&errResponse); err == nil && errResponse.Type == "error" {
		err = fmt.Errorf("HTTP Error: status=%d, code=\"%s\", message=\"%s\"", errResponse.Status, errResponse.Code, errResponse.Message)
	} else {
		err = fmt.Errorf("Error: HTTP code=%d, HTTP status=\"%s\"", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	restResp = &HarvesterResponse{
		ErrorResponse: errResponse,
		HttpResponse: resp,
	}
	return
}

// Retry Harvester API probe number of "Retries" attempts or until ready
func (c *HarvesterClient) isHarvesterApiReady() (err error) {
	var req *http.Request
	healthApi := "/ping"
	for retry := 0; retry < c.Retries; retry++ {
	        for stabilize := 0; stabilize < harvesterStabilizeMax; stabilize++ {
			if req, err = c.NewRequest("GET", healthApi, []string{}, nil); err != nil {
				break
			}
			if _, err = c.Do(req, nil); err == nil {
		                time.Sleep(harvesterStabilizeWait * time.Second)
		        } else {
		                break
		        }
		}
		if err == nil {
			return
		}
		time.Sleep(harvesterRetriesWait * time.Second)
	}
	return fmt.Errorf("harvester-client.isHarvesterReady(): harvester is not ready after %d attempts in %d seconds, last error: %s", c.Retries, harvesterRetriesWait * c.Retries, err)
}

// GetNode retrievs *corev1.Node node structure
func (c *HarvesterClient) GetNode(nodeName string) (resp *HarvesterResponse, node *corev1.Node, err error) {
	var req *http.Request
	nodeApi := HarvesterNodeApiURI + nodeName
	if err = c.isHarvesterApiReady(); err == nil {
		if req, err = c.NewRequest("GET", nodeApi, []string{}, nil); err == nil {
			if resp, err = c.Do(req, &node); err != nil {
				err = fmt.Errorf("harvester-client.GetNode() error: %s", err)
			}
		}
	}
	return
}

// IsNodeReady get node KubeletReady status
func (c *HarvesterClient) IsNodeReady(node *corev1.Node) (bool) {
	for _, condition := range node.Status.Conditions {
		if condition.Reason == "KubeletReady" && condition.Type == "Ready" {
			if condition.Status == "True" {
				return true
			}
		}
	}
	return false
}

// IsNodeInMaintainanceMode checks if node in Maintainance mode
func (c *HarvesterClient) IsNodeInMaintainanceMode(node *corev1.Node) (bool) {
	if node.Spec.Unschedulable {
		annotationValue, exists := node.Annotations[MaintainanceStatusAnnotationKey]
		if exists && annotationValue == "completed" {
			return true
		}
	}
	return false
}

// NodeEnableMaintainanceMode enables node maintainance mode
func (c *HarvesterClient) NodeEnableMaintainanceMode(nodeName string) (err error) {
	var req *http.Request
	nodeApi := HarvesterNodeApiURI + nodeName
	reqParameters := []string{"action=enableMaintenanceMode"}
	actionArgs := make(map[string]string)
	if err = c.isHarvesterApiReady(); err == nil {
		if req, err = c.NewRequest("POST", nodeApi, reqParameters, actionArgs); err == nil {
			if _, err = c.Do(req, nil); err != nil {
				err = fmt.Errorf("harvester-client.NodeEnableMaintainanceMode() error: %s", err)
			}
		}
	}
	return
}

// NodeDisableMaintainanceMode disables node maintainance mode
func (c *HarvesterClient) NodeDisableMaintainanceMode(nodeName string) (err error) {
	var req *http.Request
	nodeApi := HarvesterNodeApiURI + nodeName
	reqParameters := []string{"action=disableMaintenanceMode"}
	actionArgs := make(map[string]string)
	if err = c.isHarvesterApiReady(); err == nil {
		if req, err = c.NewRequest("POST", nodeApi, reqParameters, actionArgs); err == nil {
			if _, err = c.Do(req, nil); err != nil {
				err = fmt.Errorf("harvester-client.NodeDisableMaintainanceMode() error: %s", err)
			}
		}
	}
	return
}

// DeleteNode deletes node
func (c *HarvesterClient) DeleteNode(nodeName string) (err error) {
	var req *http.Request
	nodeApi := HarvesterNodeApiURI + nodeName
	if err = c.isHarvesterApiReady(); err == nil {
		if req, err = c.NewRequest("DELETE", nodeApi, []string{}, nil); err == nil {
			if _, err = c.Do(req, nil); err != nil {
				err = fmt.Errorf("harvester-client.DeleteNode() error: %s", err)
			}
		}
	}
	return
}
