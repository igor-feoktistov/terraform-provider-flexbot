package rancher

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	maxHTTPRedirect = 5
)

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
