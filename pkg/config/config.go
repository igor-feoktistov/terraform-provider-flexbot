package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/igor-feoktistov/go-ucsm-sdk/util"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"gopkg.in/yaml.v3"
)

const (
	imageRepoVolName    = "image_repo"
	templateRepoVolName = "template_repo"
	zapiVersion         = "1.160"
	apiMethod           = "zapi"
)

// Name convention for cDOT storage objects (can be overriden via config.yaml)
const (
	volumeNameTemplate  string = "{{.Compute.HostName}}_iboot"
	bootLunNameTemplate string = "{{.Compute.HostName}}_iboot"
	dataLunNameTemplate string = "{{.Compute.HostName}}_data"
	seedLunNameTemplate string = "{{.Compute.HostName}}_seed"
	igroupNameTemplate  string = "{{.Compute.HostName}}_iboot"
)

// Credentials is generic credentials resources
type Credentials struct {
	Host     string `yaml:"host,omitempty" json:"host,omitempty"`
	User     string `yaml:"user,omitempty" json:"user,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// InfobloxCredentials is Infoblox specific credentials
type InfobloxCredentials struct {
	Credentials `yaml:",inline" json:",inline"`
	WapiVersion string `yaml:"wapiVersion,omitempty" json:"wapiVersion,omitempty"`
	DnsView     string `yaml:"dnsView,omitempty" json:"dnsView,omitempty"`
	NetworkView string `yaml:"networkView,omitempty" json:"networkView,omitempty"`
}

// CdotCredentials is cDOT specific credentials
type CdotCredentials struct {
	Credentials `yaml:",inline" json:",inline"`
	ApiMethod   string `yaml:"apiMethod,omitempty" json:"apiMethod,omitempty"`
	ZapiVersion string `yaml:"zapiVersion,omitempty" json:"zapiVersion,omitempty"`
}

// NetworkInterface is generic network interface
type NetworkInterface struct {
	Name       string            `yaml:"name" json:"name"`
	Macaddr    string            `yaml:"macaddr,omitempty" json:"macaddr,omitempty"`
	Ip         string            `yaml:"ip,omitempty" json:"ip,omitempty"`
	Fqdn       string            `yaml:"fqdn,omitempty" json:"fqdn,omitempty"`
	Subnet     string            `yaml:"subnet" json:"subnet"`
	NetLen     string            `yaml:"netlen,omitempty" json:"netlen,omitempty"`
	IpRange    string            `yaml:"ipRange,omitempty" json:"ipRange,omitempty"`
	Gateway    string            `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DnsServer1 string            `yaml:"dnsServer1,omitempty" json:"dnsServer1,omitempty"`
	DnsServer2 string            `yaml:"dnsServer2,omitempty" json:"dnsServer2,omitempty"`
	DnsServer3 string            `yaml:"dnsServer3,omitempty" json:"dnsServer3,omitempty"`
	DnsDomain  string            `yaml:"dnsDomain,omitempty" json:"dnsDomain,omitempty"`
	Parameters map[string]string `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// IscsiTarget is iSCSI target
type IscsiTarget struct {
	NodeName   string   `yaml:"nodeName,omitempty" json:"nodeName,omitempty"`
	Interfaces []string `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

// IscsiInitiator is iSCSI initiator
type IscsiInitiator struct {
	NetworkInterface `yaml:",inline" json:",inline"`
	InitiatorName    string       `yaml:"initiatorName,omitempty" json:"initiatorName,omitempty"`
	IscsiTarget      *IscsiTarget `yaml:"iscsiTarget,omitempty" json:"iscsiTarget,omitempty"`
}

// Ipam is generic IPAM
type Ipam struct {
	Provider      string              `yaml:"provider" json:"provider"`
	IbCredentials InfobloxCredentials `yaml:"ibCredentials,omitempty" json:"ibCredentials,omitempty"`
	DnsZone       string              `yaml:"dnsZone,omitempty" json:"dnsZone,omitempty"`
}

// Compute is UCS compute
type Compute struct {
	UcsmCredentials Credentials    `yaml:"ucsmCredentials,omitempty" json:"ucsmCredentials,omitempty"`
	HostName        string         `yaml:"hostName,omitempty" json:"hostName,omitempty"`
	SpOrg           string         `yaml:"spOrg" json:"spOrg"`
	SpTemplate      string         `yaml:"spTemplate" json:"spTemplate"`
	SpDn            string         `yaml:"spDn,omitempty" json:"spDn,omitempty"`
	BladeSpec       util.BladeSpec `yaml:"bladeSpec,omitempty" json:"bladeSpec,omitempty"`
	BladeAssigned   util.BladeSpec `yaml:"bladeAssigned,omitempty" json:"bladeAssigned,omitempty"`
	Powerstate      string         `yaml:"powerState,omitempty" json:"powerState,omitempty"`
	Description     string         `yaml:"description,omitempty" json:"description,omitempty"`
	Label           string         `yaml:"label,omitempty" json:"label,omitempty"`
}

// RemoteFile is generic remote file definition
type RemoteFile struct {
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Location string `yaml:"location,omitempty" json:"location,omitempty"`
}

// Lun is cDOT LUN
type Lun struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	Id   int    `yaml:"id,omitempty" json:"id,omitempty"`
	Size int    `yaml:"size,omitempty" json:"size,omitempty"`
}

// BootLun is compute boot LUN
type BootLun struct {
	Lun     `yaml:",inline" json:",inline"`
	OsImage RemoteFile `yaml:"osImage,omitempty" json:"osImage,omitempty"`
}

// SeedLun is compute clout-init configuration LUN
type SeedLun struct {
	Lun          `yaml:",inline" json:",inline"`
	SeedTemplate RemoteFile `yaml:"seedTemplate" json:"seedTemplate"`
}

// Storage is cDOT storage
type Storage struct {
	CdotCredentials  CdotCredentials `yaml:"cdotCredentials,omitempty" json:"cdotCredentials,omitempty"`
	SvmName          string          `yaml:"svmName,omitempty" json:"svmName,omitempty"`
	ImageRepoName    string          `yaml:"imageRepoName,omitempty" json:"imageRepoName,omitempty"`
	TemplateRepoName string          `yaml:"templateRepoName,omitempty" json:"templateRepoName,omitempty"`
	VolumeName       string          `yaml:"volumeName,omitempty" json:"volumeName,omitempty"`
	IgroupName       string          `yaml:"igroupName,omitempty" json:"igroupName,omitempty"`
	BootLun          BootLun         `yaml:"bootLun,omitempty" json:"bootLun,omitempty"`
	DataLun          Lun             `yaml:"dataLun,omitempty" json:"dataLun,omitempty"`
	SeedLun          SeedLun         `yaml:"seedLun,omitempty" json:"seedLun,omitempty"`
	Snapshots        []string        `yaml:"snapshots,omitempty" json:"snapshots,omitempty"`
}

// Network is compute network
type Network struct {
	Node           []NetworkInterface `yaml:"node" json:"node"`
	IscsiInitiator []IscsiInitiator   `yaml:"iscsiInitiator" json:"iscsiInitiator"`
}

// NodeConfig is aggregated node configuration
type NodeConfig struct {
	Ipam         Ipam                               `yaml:"ipam" json:"ipam"`
	Compute      Compute                            `yaml:"compute" json:"compute"`
	Storage      Storage                            `yaml:"storage" json:"storage"`
	Network      Network                            `yaml:"network" json:"network"`
	CloudArgs    map[string]string                  `yaml:"cloudArgs,omitempty" json:"cloudArgs,omitempty"`
	Labels       map[string]string                  `yaml:"labels,omitempty" json:"labels,omitempty"`
	Taints       []rancherManagementClient.Taint    `yaml:"taints,omitempty" json:"taints,omitempty"`
	ChangeStatus uint32                             `yaml:"changeStatus,omitempty" json:"changeStatus,omitempty"`
}

// SetDefaults sets initial configuration with default values
func SetDefaults(nodeConfig *NodeConfig, hostName string, image string, templatePath string, passPhrase string) (err error) {
	var ipv4Net *net.IPNet
	if nodeConfig.Storage.CdotCredentials.ApiMethod == "" {
		nodeConfig.Storage.CdotCredentials.ApiMethod = apiMethod
	}
	if nodeConfig.Storage.CdotCredentials.ZapiVersion == "" {
		nodeConfig.Storage.CdotCredentials.ZapiVersion = zapiVersion
	}
	if nodeConfig.Storage.ImageRepoName == "" {
		nodeConfig.Storage.ImageRepoName = imageRepoVolName
	}
	if nodeConfig.Storage.TemplateRepoName == "" {
		nodeConfig.Storage.TemplateRepoName = templateRepoVolName
	}
	if hostName != "" {
		nodeConfig.Compute.HostName = hostName
	}
	if image != "" {
		nodeConfig.Storage.BootLun.OsImage.Name = image
	}
	if templatePath != "" {
		nodeConfig.Storage.SeedLun.SeedTemplate.Name = filepath.Base(templatePath)
		nodeConfig.Storage.SeedLun.SeedTemplate.Location = templatePath
	}
	if nodeConfig.Compute.HostName != "" {
		if len(nodeConfig.Network.IscsiInitiator) < 1 {
			err = fmt.Errorf("expected at least one iSCSI initiator")
			return
		}
		for i := range nodeConfig.Network.Node {
			if _, ipv4Net, err = net.ParseCIDR(nodeConfig.Network.Node[i].Subnet); err != nil {
				err = fmt.Errorf("failed to parse CIDR %s: %s", nodeConfig.Network.Node[i].Subnet, err)
				return
			}
			netLen, _ := ipv4Net.Mask.Size()
			nodeConfig.Network.Node[i].NetLen = strconv.Itoa(netLen)
		}
		for i := range nodeConfig.Network.IscsiInitiator {
			if _, ipv4Net, err = net.ParseCIDR(nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
				err = fmt.Errorf("failed to parse CIDR %s: %s", nodeConfig.Network.IscsiInitiator[i].Subnet, err)
				return
			}
			netLen, _ := ipv4Net.Mask.Size()
			nodeConfig.Network.IscsiInitiator[i].NetLen = strconv.Itoa(netLen)
			nodeConfig.Network.IscsiInitiator[i].InitiatorName = "iqn.2005-02.com.open-iscsi:" + nodeConfig.Compute.HostName + "." + strconv.Itoa(i+1)
			if nodeConfig.Network.IscsiInitiator[i].Gateway == "" {
				nodeConfig.Network.IscsiInitiator[i].Gateway = "0.0.0.0"
			}
			if nodeConfig.Network.IscsiInitiator[i].DnsServer1 == "" {
				nodeConfig.Network.IscsiInitiator[i].DnsServer1 = "0.0.0.0"
			}
			if nodeConfig.Network.IscsiInitiator[i].DnsServer2 == "" {
				nodeConfig.Network.IscsiInitiator[i].DnsServer2 = "0.0.0.0"
			}
		}
		if nodeConfig.Storage.VolumeName == "" {
			nodeConfig.Storage.VolumeName = volumeNameTemplate
		}
		if nodeConfig.Storage.IgroupName == "" {
			nodeConfig.Storage.IgroupName = igroupNameTemplate
		}
		if nodeConfig.Storage.BootLun.Name == "" {
			nodeConfig.Storage.BootLun.Name = bootLunNameTemplate
		}
		if nodeConfig.Storage.BootLun.Size == 0 {
			nodeConfig.Storage.BootLun.Size = 10
		}
		nodeConfig.Storage.BootLun.Id = 0
		if nodeConfig.Storage.DataLun.Name == "" {
			nodeConfig.Storage.DataLun.Name = dataLunNameTemplate
		}
		nodeConfig.Storage.DataLun.Id = 1
		if nodeConfig.Storage.SeedLun.Name == "" {
			nodeConfig.Storage.SeedLun.Name = seedLunNameTemplate
		}
		nodeConfig.Storage.SeedLun.Id = 2
		var tWriter bytes.Buffer
		var t *template.Template
		t = template.Must(template.New("VolumeName").Parse(nodeConfig.Storage.VolumeName))
		tWriter.Reset()
		if err = t.Execute(&tWriter, nodeConfig); err != nil {
			return
		}
		nodeConfig.Storage.VolumeName = strings.Replace(tWriter.String(), "-", "_", -1)
		t = template.Must(template.New("IgroupName").Parse(nodeConfig.Storage.IgroupName))
		tWriter.Reset()
		if err = t.Execute(&tWriter, nodeConfig); err != nil {
			return
		}
		nodeConfig.Storage.IgroupName = strings.Replace(tWriter.String(), "-", "_", -1)
		t = template.Must(template.New("BootLunName").Parse(nodeConfig.Storage.BootLun.Name))
		tWriter.Reset()
		if err = t.Execute(&tWriter, nodeConfig); err != nil {
			return
		}
		nodeConfig.Storage.BootLun.Name = strings.Replace(tWriter.String(), "-", "_", -1)
		t = template.Must(template.New("DataLunName").Parse(nodeConfig.Storage.DataLun.Name))
		tWriter.Reset()
		if err = t.Execute(&tWriter, nodeConfig); err != nil {
			return
		}
		nodeConfig.Storage.DataLun.Name = strings.Replace(tWriter.String(), "-", "_", -1)
		t = template.Must(template.New("SeedLunName").Parse(nodeConfig.Storage.SeedLun.Name))
		tWriter.Reset()
		if err = t.Execute(&tWriter, nodeConfig); err != nil {
			return
		}
		nodeConfig.Storage.SeedLun.Name = strings.Replace(tWriter.String(), "-", "_", -1)
	}
	if passPhrase != "" {
		err = DecryptNodeConfig(nodeConfig, passPhrase)
	}
	return
}

// ParseNodeConfig parses node configuration
func ParseNodeConfig(nodeConfigArg string, nodeConfig *NodeConfig) (err error) {
	var b []byte

	b = []byte(nodeConfigArg)
	if b[0] != '{' {
		if nodeConfigArg == "STDIN" {
			b, err = ioutil.ReadAll(os.Stdin)
		} else {
			b, err = ioutil.ReadFile(nodeConfigArg)
		}
	}
	if err != nil {
		err = fmt.Errorf("ParseNodeConfig: ReadFile() failure: %s", err)
		return
	}
	if b[0] == '{' {
		err = json.Unmarshal(b, nodeConfig)
	} else {
		err = yaml.Unmarshal(b, nodeConfig)
	}
	if err != nil {
		err = fmt.Errorf("ParseNodeConfig: Unmarshal() failure: %s: %s", err, string(b))
	}
	return
}

// EncryptNodeConfig encrypts node configuration
func EncryptNodeConfig(nodeConfig *NodeConfig, passPhrase string) (err error) {
	if nodeConfig.Ipam.IbCredentials.User, err = crypt.EncryptString(nodeConfig.Ipam.IbCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Ipam.IbCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Ipam.IbCredentials.Password, err = crypt.EncryptString(nodeConfig.Ipam.IbCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Ipam.IbCredentials.Password): failure: %s", err)
		return
	}
	if nodeConfig.Storage.CdotCredentials.User, err = crypt.EncryptString(nodeConfig.Storage.CdotCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Storage.CdotCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Storage.CdotCredentials.Password, err = crypt.EncryptString(nodeConfig.Storage.CdotCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Storage.CdotCredentials.Password): failure: %s", err)
		return
	}
	if nodeConfig.Compute.UcsmCredentials.User, err = crypt.EncryptString(nodeConfig.Compute.UcsmCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Compute.UcsmCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Compute.UcsmCredentials.Password, err = crypt.EncryptString(nodeConfig.Compute.UcsmCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("EncryptNodeConfig(nodeConfig.Compute.UcsmCredentials.Password): failure: %s", err)
	}
	return
}

// DecryptNodeConfig decrypts node configuration
func DecryptNodeConfig(nodeConfig *NodeConfig, passPhrase string) (err error) {
	if nodeConfig.Ipam.IbCredentials.User, err = crypt.DecryptString(nodeConfig.Ipam.IbCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Ipam.IbCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Ipam.IbCredentials.Password, err = crypt.DecryptString(nodeConfig.Ipam.IbCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Ipam.IbCredentials.Password): failure: %s", err)
		return
	}
	if nodeConfig.Storage.CdotCredentials.User, err = crypt.DecryptString(nodeConfig.Storage.CdotCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Storage.CdotCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Storage.CdotCredentials.Password, err = crypt.DecryptString(nodeConfig.Storage.CdotCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Storage.CdotCredentials.Password): failure: %s", err)
		return
	}
	if nodeConfig.Compute.UcsmCredentials.User, err = crypt.DecryptString(nodeConfig.Compute.UcsmCredentials.User, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Compute.UcsmCredentials.User): failure: %s", err)
		return
	}
	if nodeConfig.Compute.UcsmCredentials.Password, err = crypt.DecryptString(nodeConfig.Compute.UcsmCredentials.Password, passPhrase); err != nil {
		err = fmt.Errorf("DecryptNodeConfig(nodeConfig.Compute.UcsmCredentials.Password): failure: %s", err)
	}
        for argKey, argValue := range nodeConfig.CloudArgs {
		if nodeConfig.CloudArgs[argKey], err = crypt.DecryptString(argValue, passPhrase); err != nil {
			err = fmt.Errorf("DecryptNodeConfig(nodeConfig.CloudArgs[%s]): failure: %s", argKey, err)
			return
		}
        }
	return
}

// GetNodeConfigYAML transforms node configuration to YAML
func GetNodeConfigYAML(nodeConfig *NodeConfig) (b []byte, err error) {
	b, err = yaml.Marshal(nodeConfig)
	return
}

// GetNodeConfigJSON transforms node configuration to JSON
func GetNodeConfigJSON(nodeConfig *NodeConfig) (b []byte, err error) {
	b, err = json.MarshalIndent(nodeConfig, "", "  ")
	return
}
