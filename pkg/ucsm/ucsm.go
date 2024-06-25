package ucsm

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/igor-feoktistov/go-ucsm-sdk/api"
	"github.com/igor-feoktistov/go-ucsm-sdk/mo"
	"github.com/igor-feoktistov/go-ucsm-sdk/util"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
	assignTryMax  = 5
	assignWaitMax = 3600
	transientWaitMax = 300
)

// UcsmLogin return client with the authentication cookie
func UcsmLogin(endPoint string, username string, password string) (client *api.Client, err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(transientWaitMax))
        for time.Now().Before(giveupTime) {
		if client, err = util.AaaLogin(endPoint, username, password); err == nil {
			break
		}
		time.Sleep(time.Duration(30 * time.Second))
	}
	return
}

// AssignBlade assigns physical blade to SP
func AssignBlade(client *api.Client, nodeConfig *config.NodeConfig) (err error) {
	var computeBlades *[]mo.ComputeBlade
	var assignErr error
	for i := 0; i < assignTryMax; i++ {
		var pnDn string
		bladeSpec := nodeConfig.Compute.BladeSpec
		if computeBlades, err = util.ComputeBladeGetAvailable(client, &bladeSpec); err != nil {
			err = fmt.Errorf("AssignBlade: ComputeBladeGetAvailable() failure: %s", err)
			return
		}
		if len(*computeBlades) > 0 {
			rand.Seed(time.Now().UnixNano())
			pnDn = (*computeBlades)[rand.Intn(len(*computeBlades))].Dn
		} else {
			err = fmt.Errorf("AssignBlade: ComputeBladeGetAvailable(): no blades found per BladeSpec")
			return
		}
		if _, assignErr = util.SpAssignBlade(client, nodeConfig.Compute.SpDn, pnDn); assignErr == nil {
			var assocState string
			if assocState, assignErr = util.SpWaitForAssociation(client, nodeConfig.Compute.SpDn, assignWaitMax); assignErr != nil {
				err = fmt.Errorf("AssignBlade: SpWaitForAssociation() failure: %s", assignErr)
				return
			}
			if assocState == "associated" {
				var computeBlade *mo.ComputeBlade
				if computeBlade, err = util.SpGetComputeBlade(client, nodeConfig.Compute.SpDn); err != nil {
					err = fmt.Errorf("AssignBlade: SpGetComputeBlade(): %s", err)
					return
				}
				nodeConfig.Compute.BladeSpec.Dn = computeBlade.Dn
				nodeConfig.Compute.BladeAssigned = util.BladeSpec{
					Dn:           computeBlade.Dn,
					Model:        computeBlade.Model,
					Serial:       computeBlade.Serial,
					NumOfCpus:    strconv.Itoa(computeBlade.NumOfCpus),
					NumOfCores:   strconv.Itoa(computeBlade.NumOfCores),
					NumOfThreads: strconv.Itoa(computeBlade.NumOfThreads),
					TotalMemory:  strconv.Itoa(computeBlade.TotalMemory),
				}
				var vnicsEther *[]mo.VnicEther
				if vnicsEther, err = util.SpGetVnicsEther(client, nodeConfig.Compute.SpDn); err != nil {
					err = fmt.Errorf("AssignBlade: SpGetVnicsEther() failure: %s", err)
					return
				}
				for _, vnic := range *vnicsEther {
					for i := range nodeConfig.Network.Node {
						if vnic.Name == nodeConfig.Network.Node[i].Name {
							nodeConfig.Network.Node[i].Macaddr = vnic.Addr
						}
					}
				}
				return
			}
			assignErr = fmt.Errorf("AssignBlade: SpWaitForAssociation(): association state is %s", assocState)
		} else {
			assignErr = fmt.Errorf("AssignBlade: SpAssignBlade() failure: %s", assignErr)
		}
		time.Sleep(2 * time.Second)
	}
	err = assignErr
	return
}

// CreateServer creates SP from SPT
func CreateServer(nodeConfig *config.NodeConfig) (sp *mo.LsServer, err error) {
	var client *api.Client
	var lsServers []*mo.LsServer
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("CreateServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	if sp, err = util.SptInstantiate(client, nodeConfig.Compute.SpTemplate, nodeConfig.Compute.SpOrg, nodeConfig.Compute.HostName); err != nil {
		err = fmt.Errorf("CreateServer: SptInstantiate() failure: %s", err)
		return
	}
	nodeConfig.Compute.SpDn = sp.Dn
	if len(nodeConfig.Compute.Description) > 0 || len(nodeConfig.Compute.Label) > 0 {
		if _, err = util.SpSetAttributes(client, sp.Dn, nodeConfig.Compute.Description, nodeConfig.Compute.Label); err != nil {
			err = fmt.Errorf("CreateServer: SpSetAttributes() failure: %s", err)
			return
		}
	}
	if _, err = util.SpUnbindFromSpt(client, sp.Dn); err != nil {
		err = fmt.Errorf("CreateServer: SpUnbindFromSpt() failure: %s", err)
		return
	}
	var iscsiVnicAddr mo.VnicIPv4IscsiAddr
	var ipv4Net *net.IPNet
	for i := range nodeConfig.Network.IscsiInitiator[:2] {
		if _, ipv4Net, err = net.ParseCIDR(nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("CreateServer: ParseCIDR() failure for subnet %s: %s", nodeConfig.Network.IscsiInitiator[i].Subnet, err)
			return
		}
		iscsiVnicAddr = mo.VnicIPv4IscsiAddr{
			Addr:    nodeConfig.Network.IscsiInitiator[i].Ip,
			Subnet:  net.IPv4(ipv4Net.Mask[0], ipv4Net.Mask[1], ipv4Net.Mask[2], ipv4Net.Mask[3]).String(),
			DefGw:   nodeConfig.Network.IscsiInitiator[i].Gateway,
			PrimDns: nodeConfig.Network.IscsiInitiator[i].DnsServer1,
			SecDns:  nodeConfig.Network.IscsiInitiator[i].DnsServer2,
		}
		var iscsiTargets []mo.VnicIScsiStaticTargetIf
		for j := range nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces {
			if j < 2 {
				iscsiTargets = append(iscsiTargets, mo.VnicIScsiStaticTargetIf{
					IpAddress: nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces[j],
					Name:      nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName,
					Port:      "3260",
					Priority:  strconv.Itoa(j + 1),
					VnicLuns:  []mo.VnicLun{{Bootable: "yes", Id: "0"}},
				})
			}
		}
		if _, err = util.SpSetIscsiBoot(client, sp.Dn, nodeConfig.Network.IscsiInitiator[i].Name, nodeConfig.Network.IscsiInitiator[i].InitiatorName, iscsiVnicAddr, iscsiTargets); err != nil {
			err = fmt.Errorf("CreateServer: SpSetIscsiBoot() failure for iSCSI interface %s: %s", nodeConfig.Network.IscsiInitiator[i].Name, err)
			return
		}
	}
	if len(nodeConfig.Network.NvmeHost) > 0 {
	        if lsServers, err = util.ServerGet(client, nodeConfig.Compute.SpDn, "instance"); err != nil {
		        err = fmt.Errorf("CreateServer: ServerGet() failure: %s", err)
		        return
	        }
	        for i := range nodeConfig.Network.NvmeHost {
	                nodeConfig.Network.NvmeHost[i].HostNqn = "nqn.2014-08.org.nvmexpress:uuid:" + lsServers[0].Uuid
	        }
	}
	err = AssignBlade(client, nodeConfig)
	return
}

// CreateServerPreflight does sanity check before SP creation
func CreateServerPreflight(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("CreateServerPreflight: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var computeBlades *[]mo.ComputeBlade
	if computeBlades, err = util.ComputeBladeGetAvailable(client, &nodeConfig.Compute.BladeSpec); err != nil {
		err = fmt.Errorf("CreateServerPreflight: ComputeBladeGetAvailable() failure: %s", err)
		return
	}
	if len(*computeBlades) == 0 {
		err = fmt.Errorf("CreateServerPreflight: ComputeBladeGetAvailable(): no blades found per BladeSpec")
		return
	}
	var vnicsIScsi *[]mo.VnicIScsi
	if vnicsIScsi, err = util.SpGetVnicsIScsi(client, nodeConfig.Compute.SpTemplate); err != nil {
		err = fmt.Errorf("CreateServerPreflight: SpGetVnicsIScsi(): %s", err)
		return
	}
	if len(*vnicsIScsi) == 0 {
		err = fmt.Errorf("CreateServerPreflight: SpGetVnicsIScsi(): SPT \"%s\" is not configured for iSCSI boot", nodeConfig.Compute.SpTemplate)
		return
	}
	var vnicsEther *[]mo.VnicEther
	if vnicsEther, err = util.SpGetVnicsEther(client, nodeConfig.Compute.SpTemplate); err != nil {
		err = fmt.Errorf("CreateServerPreflight: SpGetVnicsEther(): %s", err)
		return
	}
	if len(*vnicsEther) == 0 {
		err = fmt.Errorf("CreateServerPreflight: SpGetVnicsEther(): no ethernet vNICs found in SPT \"%s\"", nodeConfig.Compute.SpTemplate)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator[:2] {
		if _, _, err = net.ParseCIDR(nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("CreateServerPreflight: ParseCIDR(): failure for subnet %s: %s", nodeConfig.Network.IscsiInitiator[i].Subnet, err)
			return
		}
		var found int = 0
		for _, vnic := range *vnicsIScsi {
			if vnic.Name == nodeConfig.Network.IscsiInitiator[i].Name {
				found++
			}
		}
		if found == 0 {
			err = fmt.Errorf("CreateServerPreflight: no iSCSI vNICs found in SPT \"%s\" that match iscsiInitiator interface \"%s\"", nodeConfig.Compute.SpTemplate, nodeConfig.Network.IscsiInitiator[i].Name)
		}
	}
	for _, node := range nodeConfig.Network.Node {
		if _, _, err = net.ParseCIDR(node.Subnet); err != nil {
			err = fmt.Errorf("CreateServerPreflight: ParseCIDR(): failure for node subnet %s: %s", node.Subnet, err)
			return
		}
		var found int = 0
		for _, vnic := range *vnicsEther {
			if vnic.Name == node.Name {
				found++
			}
		}
		if found == 0 {
			err = fmt.Errorf("CreateServerPreflight: no ethernet vNICs found in SPT \"%s\" that match node interface \"%s\"", nodeConfig.Compute.SpTemplate, node.Name)
		}
	}
	return
}

// DiscoverServer finds SP by name and retrives it's attributes
func DiscoverServer(nodeConfig *config.NodeConfig) (serverExists bool, err error) {
	var client *api.Client
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("DiscoverServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var lsServers []*mo.LsServer
	nodeConfig.Compute.SpDn = nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if lsServers, err = util.ServerGet(client, nodeConfig.Compute.SpDn, "instance"); err != nil {
		err = fmt.Errorf("DiscoverServer: ServerGet() failure: %s", err)
		return
	}
	if len(lsServers) == 0 {
		serverExists = false
		return
	}
	serverExists = true
	var computeBlade *mo.ComputeBlade
	if computeBlade, err = util.SpGetComputeBlade(client, nodeConfig.Compute.SpDn); err != nil {
		err = fmt.Errorf("DiscoverServer: SpGetComputeBlade(): %s", err)
		return
	}
	if computeBlade == nil {
		computeBlade = &mo.ComputeBlade{}
	}
	nodeConfig.Compute.BladeSpec.Dn = computeBlade.Dn
	nodeConfig.Compute.BladeAssigned = util.BladeSpec{
		Dn:           computeBlade.Dn,
		Model:        computeBlade.Model,
		Serial:       computeBlade.Serial,
		NumOfCpus:    strconv.Itoa(computeBlade.NumOfCpus),
		NumOfCores:   strconv.Itoa(computeBlade.NumOfCores),
		NumOfThreads: strconv.Itoa(computeBlade.NumOfThreads),
		TotalMemory:  strconv.Itoa(computeBlade.TotalMemory),
	}
	var vnicsEther *[]mo.VnicEther
	if vnicsEther, err = util.SpGetVnicsEther(client, nodeConfig.Compute.SpDn); err != nil {
		err = fmt.Errorf("DiscoverServer: SpGetVnicsEther(): %s", err)
		return
	}
	if len(*vnicsEther) == 0 {
		err = fmt.Errorf("DiscoverServer: SpGetVnicsEther(): no ethernet vNICs found in SP \"%s\"", nodeConfig.Compute.SpDn)
		return
	}
	for i := range nodeConfig.Network.Node {
		var found int = 0
		for _, vnic := range *vnicsEther {
			if vnic.Name == nodeConfig.Network.Node[i].Name {
				nodeConfig.Network.Node[i].Macaddr = vnic.Addr
				found++
			}
		}
		if found == 0 {
			err = fmt.Errorf("DiscoverServer: no ethernet vNICs found in SP \"%s\" that match node interface \"%s\"", nodeConfig.Compute.SpDn, nodeConfig.Network.Node[i].Name)
			return
		}
	}
	var vnicsIScsi *[]mo.VnicIScsi
	if vnicsIScsi, err = util.SpGetVnicsIScsi(client, nodeConfig.Compute.SpDn); err != nil {
		err = fmt.Errorf("DiscoverServer: SpGetVnicsIScsi(): %s", err)
		return
	}
	if len(*vnicsIScsi) == 0 {
		err = fmt.Errorf("DiscoverServer: SpGetVnicsIScsi(): SP \"%s\" is not configured for iSCSI boot", nodeConfig.Compute.SpDn)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator[:2] {
		var found int = 0
		for _, vnic := range *vnicsIScsi {
			if vnic.Name == nodeConfig.Network.IscsiInitiator[i].Name {
				if len(vnic.VnicVlan.VnicIScsiStaticTargets) > 0 {
					nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
					for _, target := range vnic.VnicVlan.VnicIScsiStaticTargets {
						nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = target.Name
						nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, target.IpAddress)
					}
					nodeConfig.Network.IscsiInitiator[i].Ip = vnic.VnicVlan.VnicIPv4If.VnicIPv4IscsiAddr.Addr
					nodeConfig.Network.IscsiInitiator[i].Gateway = vnic.VnicVlan.VnicIPv4If.VnicIPv4IscsiAddr.DefGw
					nodeConfig.Network.IscsiInitiator[i].DnsServer1 = vnic.VnicVlan.VnicIPv4If.VnicIPv4IscsiAddr.PrimDns
					nodeConfig.Network.IscsiInitiator[i].DnsServer2 = vnic.VnicVlan.VnicIPv4If.VnicIPv4IscsiAddr.SecDns
				} else {
					err = fmt.Errorf("DiscoverServer: SpGetVnicsIScsi(): iSCSI targets are not configured for interface \"%s\" in SP \"%s\"", nodeConfig.Network.IscsiInitiator[i].Name, nodeConfig.Compute.SpDn)
					return
				}
				found++
			}
		}
		if found == 0 {
			err = fmt.Errorf("DiscoverServer: no iSCSI vNICs found in iscsiInitiator configuration that match iSCSI vNIC \"%s\"", nodeConfig.Network.IscsiInitiator[i].Name)
			return
		}
	}
	for i := range nodeConfig.Network.NvmeHost {
	        nodeConfig.Network.NvmeHost[i].HostNqn = "nqn.2014-08.org.nvmexpress:uuid:" + lsServers[0].Uuid
	}
	nodeConfig.Compute.Description = lsServers[0].Descr
	nodeConfig.Compute.Label = lsServers[0].UsrLbl
	if lsServers[0].AssignState == "assigned" {
		nodeConfig.Compute.BladeSpec.Dn = lsServers[0].PnDn
	} else {
		if err = AssignBlade(client, nodeConfig); err != nil {
			return
		}
	}
	if nodeConfig.Compute.Powerstate, err = util.SpGetPowerState(client, nodeConfig.Compute.SpDn); err != nil {
		err = fmt.Errorf("DiscoverServer: SpGetPowerState() failure: %s", err)
	}
	return
}

// UpdateServerPreflight check blade availability before re-assigning
func UpdateServerPreflight(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("UpdateServerPreflight: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var computeBlades *[]mo.ComputeBlade
	if computeBlades, err = util.ComputeBladeGetAvailable(client, &nodeConfig.Compute.BladeSpec); err != nil {
		err = fmt.Errorf("UpdateServerPreflight: ComputeBladeGetAvailable() failure: %s", err)
		return
	}
	if len(*computeBlades) == 0 {
		err = fmt.Errorf("UpdateServerPreflight: ComputeBladeGetAvailable(): no blades found per BladeSpec")
		return
	}
	return
}

// UpdateServer re-assigns physical blade
func UpdateServer(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("UpdateServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var lsServers []*mo.LsServer
	nodeConfig.Compute.SpDn = nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if lsServers, err = util.ServerGet(client, nodeConfig.Compute.SpDn, "instance"); err != nil {
		err = fmt.Errorf("UpdateServer: ServerGet() failure: %s", err)
		return
	}
	if len(lsServers) == 0 {
		err = fmt.Errorf("UpdateServer: ServerGet() failure: server does not exist")
		return
	}
	var powerState string
	if powerState, err = util.SpGetPowerState(client, nodeConfig.Compute.SpDn); err != nil {
		err = fmt.Errorf("UpdateServer: SpGetPowerState() failure: %s", err)
		return
	}
	if powerState == "up" {
		err = fmt.Errorf("UpdateServer: power state is \"up\", cannot re-assign blade")
		return
	}
	err = AssignBlade(client, nodeConfig)
	return
}

// UpdateServerAttributes sets updates SP description and label
func UpdateServerAttributes(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password)
	if err != nil {
		err = fmt.Errorf("UpdateServerAttributes: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var lsServers []*mo.LsServer
	nodeConfig.Compute.SpDn = nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if lsServers, err = util.ServerGet(client, nodeConfig.Compute.SpDn, "instance"); err != nil {
		err = fmt.Errorf("UpdateServerAttributes: ServerGet() failure: %s", err)
		return
	}
	if len(lsServers) == 0 {
		err = fmt.Errorf("UpdateServerAttributes: ServerGet() failure: server does not exist")
		return
	}
	if lsServers[0].Descr != nodeConfig.Compute.Description || lsServers[0].UsrLbl != nodeConfig.Compute.Label {
		if _, err = util.SpSetAttributes(client, nodeConfig.Compute.SpDn, nodeConfig.Compute.Description, nodeConfig.Compute.Label); err != nil {
			err = fmt.Errorf("UpdateServerAttributes: SpSetAttributes() failure: %s", err)
		}
	}
	return
}

// DeleteServer deletes SP
func DeleteServer(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	var powerState string
	spDn := nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password); err != nil {
		err = fmt.Errorf("DeleteServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	var lsServers []*mo.LsServer
	if lsServers, err = util.ServerGet(client, spDn, "instance"); err != nil {
		err = fmt.Errorf("DeleteServer: ServerGet() failure: %s", err)
		return
	}
	if len(lsServers) > 0 {
		if powerState, err = util.SpGetPowerState(client, spDn); err != nil {
			err = fmt.Errorf("DeleteServer: SpGetPowerState() failure: %s", err)
			return
		}
		if powerState == "down" {
			if err = util.SpDelete(client, spDn); err != nil {
				err = fmt.Errorf("DeleteServer: SpDelete() failure: %s", err)
			}
		} else {
			if powerState == "" {
				err = fmt.Errorf("DeleteServer: server \"%s\" not found", spDn)
			} else {
				err = fmt.Errorf("DeleteServer: server \"%s\" has power state \"%s\"", spDn, powerState)
			}
		}
	}
	return
}

// StartServer sets SP powerstate to "up"
func StartServer(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	var lsPower *mo.LsPower
	spDn := nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password); err != nil {
		err = fmt.Errorf("StartServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	if lsPower, err = util.SpSetPowerState(client, spDn, "up"); err != nil {
		err = fmt.Errorf("StartServer: SpSetPowerState() failure: %s", err)
	} else {
		nodeConfig.Compute.Powerstate = lsPower.State
	}
	return
}

// StopServer sets SP powerstate to "down"
func StopServer(nodeConfig *config.NodeConfig) (err error) {
	var client *api.Client
	var lsPower *mo.LsPower
	spDn := nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password); err != nil {
		err = fmt.Errorf("StopServer: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	if lsPower, err = util.SpSetPowerState(client, spDn, "down"); err != nil {
		err = fmt.Errorf("StopServer: SpSetPowerState() failure: %s", err)
	} else {
		nodeConfig.Compute.Powerstate = lsPower.State
	}
	return
}

// GetServerPowerState retrievs SP powerstate
func GetServerPowerState(nodeConfig *config.NodeConfig) (powerState string, err error) {
	var client *api.Client
	spDn := nodeConfig.Compute.SpOrg + "/ls-" + nodeConfig.Compute.HostName
	if client, err = UcsmLogin("https://"+nodeConfig.Compute.UcsmCredentials.Host+"/", nodeConfig.Compute.UcsmCredentials.User, nodeConfig.Compute.UcsmCredentials.Password); err != nil {
		err = fmt.Errorf("GetServerPowerState: AaaLogin() failure: %s", err)
		return
	}
	defer client.AaaLogout()
	powerState, err = util.SpGetPowerState(client, spDn)
	return
}
