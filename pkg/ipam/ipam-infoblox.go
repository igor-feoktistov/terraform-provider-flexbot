package ipam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	ibclient "github.com/infobloxopen/infoblox-go-client"
)

type IpPool struct {
	Ips []string `json:"ips,omitempty"`
}

type HostIpAddress struct {
	IpAddress string   `json:"ip_address,omitempty"`
	Names     []string `json:"names,omitempty"`
}

type HostRecord struct {
	Ipv4Addrs []struct {
		Ipv4Addr string `json:"ipv4addr,omitempty"`
	} `json:"ipv4addrs,omitempty"`
	Name string `json:"names,omitempty"`
}

type IpRange struct {
	Ref         string `json:"_ref,omitempty"`
	Network     string `json:"network,omitempty"`
	NetworkView string `json:"network_view,omitempty"`
	StartAddr   string `json:"start_addr,omitempty"`
	EndAddr     string `json:"end_addr,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type InfobloxProvider struct {
	HostConfig  ibclient.HostConfig
	DnsView     string
	NetworkView string
	DnsZone     string
}

func validateIpRange(c *ibclient.Connector, networkView string, networkCidr string, rangeStr string) (err error) {
	var re *regexp.Regexp
	re = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)\s*-\s*(\d+\.\d+\.\d+\.\d+)`)
	var subMatch, path []string
	var startAddr, endAddr string
        subMatch = re.FindStringSubmatch(rangeStr)
        if len(subMatch) == 3 {
		startAddr = subMatch[1]
		endAddr = subMatch[2]
	} else {
		err = fmt.Errorf("validateIpRange(): unexpected IP range format: %s", rangeStr)
		return
	}
	path = []string{"wapi", "v" + c.HostConfig.Version, "range"}
	var u url.URL
	u = url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "start_addr=" + startAddr + "&end_addr=" + endAddr + "&network_view=" + networkView,
	}
	var req *http.Request
	if req, err = http.NewRequest(http.MethodGet, u.String(), nil); err != nil {
		err = fmt.Errorf("validateIpRange(): %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	var res []byte
	if res, err = c.Requestor.SendRequest(req); err != nil {
		err = fmt.Errorf("validateIpRange(): %s", err)
		return
	}
	var ipRange []IpRange
	if err = json.Unmarshal(res, &ipRange); err != nil {
		err = fmt.Errorf("validateIpRange(): Unmarshal(): %s", string(res))
		return
	}
	if !(len(ipRange) > 0 && len(ipRange[0].Network) > 0) {
		err = fmt.Errorf("validateIpRange(): IP range \"%s\" not found", rangeStr)
		return
	}
	if ipRange[0].Network != networkCidr {
		err = fmt.Errorf("validateIpRange(): IP range \"%s\" does not belong to subnet \"%s\"", rangeStr, networkCidr)
		return
	}
	re = regexp.MustCompile(`(range/\w+):\d+\.\d+\.\d+\.\d+.+`)
	subMatch = re.FindStringSubmatch(ipRange[0].Ref)
	if subMatch == nil {
		err = fmt.Errorf("validateIpRange(): missing range ref in response")
		return
	}
	path = []string{"wapi", "v" + c.HostConfig.Version, subMatch[1]}
	u = url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "_function=next_available_ip&num=1",
	}
	if req, err = http.NewRequest(http.MethodPost, u.String(), nil); err != nil {
		err = fmt.Errorf("validateIpRange(): %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	if res, err = c.Requestor.SendRequest(req); err != nil {
		err = fmt.Errorf("validateIpRange(): %s", err)
		return
	}
	var ipPool IpPool
	if err = json.Unmarshal(res, &ipPool); err != nil {
		err = fmt.Errorf("validateIpRange(): Unmarshal(): %s", string(res))
		return
	}
        if len(ipPool.Ips) == 0 {
    		err = fmt.Errorf("validateIpRange(): no IPs available in IP range \"%s\"", rangeStr)
	}
	return
}

func getAvailableIPs(conn *ibclient.Connector, network_view string, cidr string, numIPs int) (ippool IpPool, err error) {
	objMgr := ibclient.NewObjectManager(conn, "flexbot", "admin")
	var network *ibclient.Network
	if network, err = objMgr.GetNetwork(network_view, cidr, nil); err != nil {
		return
	}
	if network == nil {
		err = fmt.Errorf("network %s not found", cidr)
		return
	}
	r := regexp.MustCompile(`(network/\w+):\d+\.\d+\.\d+\.\d+/\d+/.+`)
	m := r.FindStringSubmatch(network.Ref)
	if m == nil {
		err = fmt.Errorf("missing network ref in response")
		return
	}
	path := []string{"wapi", "v" + conn.HostConfig.Version, m[1]}
	u := url.URL{
		Scheme:   "https",
		Host:     conn.HostConfig.Host + ":" + conn.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "_function=next_available_ip&num=" + strconv.Itoa(numIPs),
	}
	var req *http.Request
	if req, err = http.NewRequest(http.MethodPost, u.String(), nil); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(conn.HostConfig.Username, conn.HostConfig.Password)
	var res []byte
	if res, err = conn.Requestor.SendRequest(req); err != nil {
		return
	}
	if err = json.Unmarshal(res, &ippool); err != nil {
		err = fmt.Errorf("Unmarshal(): %s", string(res))
	}
	return
}

func getHostByIp(c *ibclient.Connector, network_view string, ipaddr string) (fqdn string, err error) {
	path := []string{"wapi", "v" + c.HostConfig.Version, "ipv4address"}
	u := url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "ip_address=" + ipaddr + "&network_view=" + network_view,
	}
	var req *http.Request
	if req, err = http.NewRequest(http.MethodGet, u.String(), nil); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	var res []byte
	if res, err = c.Requestor.SendRequest(req); err != nil {
		return
	}
	var hostaddr []HostIpAddress
	if err = json.Unmarshal(res, &hostaddr); err != nil {
		err = fmt.Errorf("%s", string(res))
		return
	}
	if len(hostaddr) > 0 && len(hostaddr[0].Names) > 0 {
		fqdn = hostaddr[0].Names[0]
	}
	return
}

func getIpByHost(c *ibclient.Connector, network_view string, fqdn string) (ipaddr string, err error) {
	path := []string{"wapi", "v" + c.HostConfig.Version, "record:host"}
	u := url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "name=" + fqdn + "&network_view=" + network_view,
	}
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	var res []byte
	res, err = c.Requestor.SendRequest(req)
	if err != nil {
		return
	}
	var host []HostRecord
	if err = json.Unmarshal(res, &host); err != nil {
		err = fmt.Errorf("%s: %s", err, string(res))
	} else {
		if len(host) > 0 && len(host[0].Ipv4Addrs) > 0 {
			ipaddr = host[0].Ipv4Addrs[0].Ipv4Addr
		}
	}
	return
}

func NewInfobloxProvider(ipam *config.Ipam) (provider *InfobloxProvider) {
	provider = &InfobloxProvider{
		HostConfig: ibclient.HostConfig{
			Host:     ipam.IbCredentials.Host,
			Version:  ipam.IbCredentials.WapiVersion,
			Port:     "443",
			Username: ipam.IbCredentials.User,
			Password: ipam.IbCredentials.Password,
		},
		DnsView:     ipam.IbCredentials.DnsView,
		NetworkView: ipam.IbCredentials.NetworkView,
		DnsZone:     ipam.DnsZone,
	}
	return
}

func (p *InfobloxProvider) AllocateIp(cidr string, fqdn string) (ipaddr string, err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("AllocateIP: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	if fqdn == "" {
		var ipPool IpPool
		if ipPool, err = getAvailableIPs(conn, p.NetworkView, cidr, 1); err != nil {
			err = fmt.Errorf("AllocateIP: getAvailableIPs(): %s", err)
		} else {
			if len(ipPool.Ips) > 0 {
				ipaddr = ipPool.Ips[0]
			} else {
				err = fmt.Errorf("AllocateIP: getAvailableIPs(): no IPs available in network %s", cidr)
			}
		}
	} else {
		recordHostIpAddr := ibclient.NewHostRecordIpv4Addr(ibclient.HostRecordIpv4Addr{})
		recordHostIpAddr.Ipv4Addr = fmt.Sprintf("func:nextavailableip:%s,%s", cidr, p.NetworkView)
		recordHostIpAddrSlice := []ibclient.HostRecordIpv4Addr{*recordHostIpAddr}
		enableDNS := new(bool)
		*enableDNS = true
		host := ibclient.NewHostRecord(ibclient.HostRecord{
			Name:        fqdn,
			EnableDns:   enableDNS,
			NetworkView: p.NetworkView,
			View:        p.DnsView,
			Ipv4Addrs:   recordHostIpAddrSlice,
		})
		var ref string
		if ref, err = conn.CreateObject(host); err != nil {
			err = fmt.Errorf("AllocateIp: CreateObject(): %s", err)
			return
		}
		host.Ref = ref
		if err = conn.GetObject(host, ref, &host); err != nil {
			err = fmt.Errorf("AllocateIp: GetObject(): %s", err)
		} else {
			ipaddr = host.Ipv4Addrs[0].Ipv4Addr
		}
	}
	return
}

func (p *InfobloxProvider) AssignIp(ipaddr string, fqdn string) (err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("AssignIP: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	recordHostIpAddr := ibclient.NewHostRecordIpv4Addr(ibclient.HostRecordIpv4Addr{})
	recordHostIpAddr.Ipv4Addr = ipaddr
	recordHostIpAddrSlice := []ibclient.HostRecordIpv4Addr{*recordHostIpAddr}
	enableDNS := new(bool)
	*enableDNS = true
	host := ibclient.NewHostRecord(ibclient.HostRecord{
		Name:        fqdn,
		EnableDns:   enableDNS,
		NetworkView: p.NetworkView,
		View:        p.DnsView,
		Ipv4Addrs:   recordHostIpAddrSlice,
	})
	if _, err = conn.CreateObject(host); err != nil {
		err = fmt.Errorf("AssignIp: CreateObject(): %s", err)
	}
	return
}

func (p *InfobloxProvider) ReleaseIp(fqdn string) (ipaddr string, err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("ReleaseIP: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	objMgr := ibclient.NewObjectManager(conn, "flexbot", "admin")
	var host *ibclient.HostRecord
	if host, err = objMgr.GetHostRecord(fqdn, p.NetworkView, "", ""); err != nil {
		err = fmt.Errorf("ReleaseIP: GetHostRecord(): %s", err)
		return
	}
	if host != nil {
		ipaddr = host.Ipv4Addrs[0].Ipv4Addr
		if _, err := objMgr.DeleteHostRecord(host.Ref); err != nil {
			err = fmt.Errorf("ReleaseIP: DeleteHostRecord(): %s", err)
		}
	}
	return
}

func (p *InfobloxProvider) Allocate(nodeConfig *config.NodeConfig) (err error) {
	var ipaddr string
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		if len(nodeConfig.Network.Node[i].Ip) > 0 {
			ipaddr = nodeConfig.Network.Node[i].Ip
			err = p.AssignIp(ipaddr, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
		} else {
			if len(nodeConfig.Network.Node[i].IpRange) > 0 {
				ipaddr, err = p.AllocateIp(nodeConfig.Network.Node[i].IpRange, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			} else {
				ipaddr, err = p.AllocateIp(nodeConfig.Network.Node[i].Subnet, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			}
		}
		if err != nil {
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		nodeConfig.Network.Node[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if len(nodeConfig.Network.IscsiInitiator[i].Ip) > 0 {
			ipaddr = nodeConfig.Network.IscsiInitiator[i].Ip
			err = p.AssignIp(ipaddr, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
		} else {
			if len(nodeConfig.Network.IscsiInitiator[i].IpRange) > 0 {
				ipaddr, err = p.AllocateIp(nodeConfig.Network.IscsiInitiator[i].IpRange, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			} else {
				ipaddr, err = p.AllocateIp(nodeConfig.Network.IscsiInitiator[i].Subnet, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			}
		}
		if err != nil {
			return
		}
		nodeConfig.Network.IscsiInitiator[i].Ip = ipaddr
		nodeConfig.Network.IscsiInitiator[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
	}
	return
}

func (p *InfobloxProvider) Discover(nodeConfig *config.NodeConfig) (err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("Discover: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		var ipaddr string
		if ipaddr, err = getIpByHost(conn, p.NetworkView, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone); err != nil {
			err = fmt.Errorf("Discover: getIpByHost(): %s", err)
			return
		}
		if ipaddr == "" {
			err = fmt.Errorf("Discover: getIpByHost(): no host record found for FQDN  %s", nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		nodeConfig.Network.Node[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if nodeConfig.Network.IscsiInitiator[i].Ip != "" {
			var fqdn string
			if fqdn, err = getHostByIp(conn, p.NetworkView, nodeConfig.Network.IscsiInitiator[i].Ip); err != nil {
				err = fmt.Errorf("Discover: getHostByIp(): %s", err)
				return
			}
			if fqdn == "" {
				err = fmt.Errorf("Discover: getHostByIp(): no host record found for IP %s", nodeConfig.Network.IscsiInitiator[i].Ip)
				return
			}
			if fqdn == nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone {
				nodeConfig.Network.IscsiInitiator[i].Fqdn = fqdn
			} else {
				err = fmt.Errorf("Discover: expected iSCSI initiator interface FQDN \"%s\", resolved \"%s\"", nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone, fqdn)
				return
			}
		}
	}
	return
}

func (p *InfobloxProvider) AllocatePreflight(nodeConfig *config.NodeConfig) (err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("Discover: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	for i, _ := range nodeConfig.Network.Node {
		if len(nodeConfig.Network.Node[i].IpRange) > 0 {
			err = validateIpRange(conn, p.NetworkView, nodeConfig.Network.Node[i].Subnet, nodeConfig.Network.Node[i].IpRange)
		} else {
			_, err = p.AllocateIp(nodeConfig.Network.Node[i].Subnet, "")
		}
		if err != nil {
			return
		}
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		if len(nodeConfig.Network.IscsiInitiator[i].IpRange) > 0 {
			err = validateIpRange(conn, p.NetworkView, nodeConfig.Network.IscsiInitiator[i].Subnet, nodeConfig.Network.IscsiInitiator[i].IpRange)
		} else {
			_, err = p.AllocateIp(nodeConfig.Network.IscsiInitiator[i].Subnet, "")
		}
		if err != nil {
			return
		}
	}
	return
}

func (p *InfobloxProvider) Release(nodeConfig *config.NodeConfig) (err error) {
	var ipaddr string
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		if ipaddr, err = p.ReleaseIp(nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if ipaddr, err = p.ReleaseIp(nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.IscsiInitiator[i].Ip = ipaddr
	}
	return
}
