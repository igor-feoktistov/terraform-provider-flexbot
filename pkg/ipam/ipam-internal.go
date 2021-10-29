package ipam

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

// InternalProvider is IPAM provider "Internal"
type InternalProvider struct{}

// NewInternalProvider initializes Internal IPAM provider
func NewInternalProvider(ipam *config.Ipam) (provider *InternalProvider) {
	provider = &InternalProvider{}
	return
}

// AllocateIp allocates IP in Internal provider
func (p *InternalProvider) AllocateIp(cidr string, fqdn string) (ipaddr string, err error) {
	return
}

// AssignIp assigns IP in Internal provider
func (p *InternalProvider) AssignIp(ipaddr string, fqdn string) (err error) {
	return
}

// ReleaseIp releases IP in Internal provider
func (p *InternalProvider) ReleaseIp(fqdn string) (ipaddr string, err error) {
	return
}

// Allocate allocates and assigns IP for compute node
func (p *InternalProvider) Allocate(nodeConfig *config.NodeConfig) (err error) {
	for i := range nodeConfig.Network.Node {
		if nodeConfig.Network.Node[i].Ip == "" {
			err = fmt.Errorf("Allocate: expected network.node[%d].ip in configuration", i)
			return
		}
		if nodeConfig.Network.Node[i].Fqdn == "" {
			err = fmt.Errorf("Allocate: expected network.node[%d].fqdn in configuration", i)
			return
		}
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		if nodeConfig.Network.IscsiInitiator[i].Ip == "" {
			err = fmt.Errorf("Allocate: expected network.iscsiinitiator[%d].ip in configuration", i)
			return
		}
		if nodeConfig.Network.IscsiInitiator[i].Fqdn == "" {
			err = fmt.Errorf("Allocate: expected network.iscsiinitiator[%d].fqdn in configuration", i)
			return
		}
	}
	return
}

// Discover discovers IP's for compute node
func (p *InternalProvider) Discover(nodeConfig *config.NodeConfig) (err error) {
	err = p.Allocate(nodeConfig)
	return
}

// AllocatePreflight is sanity check before allocation happens
func (p *InternalProvider) AllocatePreflight(nodeConfig *config.NodeConfig) (err error) {
	err = p.Allocate(nodeConfig)
	return
}

// Release releases IP's for compute node
func (p *InternalProvider) Release(nodeConfig *config.NodeConfig) (err error) {
	return
}
