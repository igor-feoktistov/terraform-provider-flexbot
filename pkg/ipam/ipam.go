package ipam

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

// IpamProvider is generic IPAM provider interface
type IpamProvider interface {
	AllocateIp(cidr string, fqdn string) (string, error)
	AssignIp(ipaddr string, fqdn string) error
	ReleaseIp(fqdn string) (string, error)
	Allocate(nodeConfig *config.NodeConfig) error
	AllocatePreflight(nodeConfig *config.NodeConfig) error
	Discover(nodeConfig *config.NodeConfig) error
	Release(nodeConfig *config.NodeConfig) error
}

// NewProvider initializes IPAM provider
func NewProvider(ipam *config.Ipam) (provider IpamProvider, err error) {
	switch ipam.Provider {
	case "Infoblox":
		provider = NewInfobloxProvider(ipam)
	case "Internal":
		provider = NewInternalProvider(ipam)
	default:
		err = fmt.Errorf("NewProvider(): IPAM provider %s is not implemented", ipam.Provider)
	}
	return
}
