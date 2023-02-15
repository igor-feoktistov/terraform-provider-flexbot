package ontap

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

// CreateNvmeStorage creates node NVME data storage in cDOT
func CreateNvmeStorage(nodeConfig *config.NodeConfig) (err error) {
        if len(nodeConfig.Network.NvmeHost) > 0 && nodeConfig.Storage.DataNvme.Size > 0 {
	        var c client.OntapClient
	        errorFormat := "CreateNvmeStorage(): %s"
	        if c, err = client.NewOntapClient(nodeConfig); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        dataNvmeNamespacePath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataNvme.Namespace
	        var namespaceExists, subsystemExists, namespaceMapped bool
	        if namespaceExists, err = c.NvmeNamespaceExists(dataNvmeNamespacePath); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        if namespaceExists {
	                if namespaceMapped, err = c.IsNvmeNamespaceMapped(dataNvmeNamespacePath); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
	                }
	                if namespaceMapped {
	                        if err = c.NvmeNamespaceUnmap(dataNvmeNamespacePath); err != nil {
		                        err = fmt.Errorf(errorFormat, err)
		                        return
	                        }
	                }
	                if err = c.NvmeNamespaceDestroy(dataNvmeNamespacePath); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
	                }
	        }
	        if c.NvmeNamespaceCreate(dataNvmeNamespacePath, nodeConfig.Storage.DataNvme.Size); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
                }
	        if subsystemExists, err = c.NvmeSubsystemExists(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        if subsystemExists {
	                if err = c.NvmeSubsystemDestroy(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
		        }
	        }
	        if err = c.NvmeSubsystemCreate(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
	                err = fmt.Errorf(errorFormat, err)
		        return
	        }
                if err = c.NvmeSubsystemAddHost(nodeConfig.Storage.DataNvme.Subsystem, nodeConfig.Network.NvmeHost[0].HostNqn); err != nil {
	                err = fmt.Errorf(errorFormat, err)
		        return
                }
	        var targetNqn string
	        if targetNqn, err = c.NvmeTargetGetNqn(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
	                err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        for i := range nodeConfig.Network.NvmeHost {
		        var lifs []string
		        if lifs, err = c.DiscoverNvmeLIFs(dataNvmeNamespacePath, nodeConfig.Network.NvmeHost[i].Subnet); err != nil {
	                        err = fmt.Errorf(errorFormat, err)
			        return
		        }
		        nodeConfig.Network.NvmeHost[i].NvmeTarget = &config.NvmeTarget{}
		        nodeConfig.Network.NvmeHost[i].NvmeTarget.TargetNqn = targetNqn
		        nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces = append(nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces, lifs...)
	        }
	}
	return
}

// CreateNvmeStoragePreflight is sanity check before actual storage provisioning
func CreateNvmeStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
        if len(nodeConfig.Network.NvmeHost) > 0 && nodeConfig.Storage.DataNvme.Size > 0 {
	        var c client.OntapClient
	        errorFormat := "CreateNvmeStoragePreflight(): %s"
	        if c, err = client.NewOntapClient(nodeConfig); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
		if _, err = c.GetNvmeLIFs(); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	return
}

// DeleteNvmeStorage deletes node NVME storage
func DeleteNvmeStorage(nodeConfig *config.NodeConfig) (err error) {
        if len(nodeConfig.Network.NvmeHost) > 0 && nodeConfig.Storage.DataNvme.Size > 0 {
	        var c client.OntapClient
	        errorFormat := "DeleteNvmeStorage(): %s"
	        if c, err = client.NewOntapClient(nodeConfig); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        dataNvmeNamespacePath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataNvme.Namespace
	        var namespaceExists, subsystemExists, namespaceMapped bool
	        if namespaceExists, err = c.NvmeNamespaceExists(dataNvmeNamespacePath); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        if namespaceExists {
	                if namespaceMapped, err = c.IsNvmeNamespaceMapped(dataNvmeNamespacePath); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
	                }
	                if namespaceMapped {
	                        if err = c.NvmeNamespaceUnmap(dataNvmeNamespacePath); err != nil {
		                        err = fmt.Errorf(errorFormat, err)
		                        return
	                        }
	                }
	                if err = c.NvmeNamespaceDestroy(dataNvmeNamespacePath); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
	                }
	        }
	        if subsystemExists, err = c.NvmeSubsystemExists(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        if subsystemExists {
	                if err = c.NvmeSubsystemDestroy(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
		        }
	        }
	}
	return
}

// DiscoverNvmeStorage discovers NVME storage in cDOT
func DiscoverNvmeStorage(nodeConfig *config.NodeConfig) (err error) {
        if len(nodeConfig.Network.NvmeHost) > 0 && nodeConfig.Storage.DataNvme.Size > 0 {
	        var c client.OntapClient
	        errorFormat := "DiscoverNvmeStorage(): %s"
	        if c, err = client.NewOntapClient(nodeConfig); err != nil {
		        err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		        return
	        }
	        dataNvmeNamespacePath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataNvme.Namespace
	        var namespaceExists bool
	        if namespaceExists, err = c.NvmeNamespaceExists(dataNvmeNamespacePath); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
	        }
	        if namespaceExists {
	                var namespaceInfo *client.NvmeNamespaceInfo
	                if namespaceInfo, err = c.NvmeNamespaceGetInfo(dataNvmeNamespacePath); err != nil {
		                err = fmt.Errorf(errorFormat, err)
		                return
	                }
	                nodeConfig.Storage.DataNvme.Size = namespaceInfo.Size
	                var targetNqn string
	                if targetNqn, err = c.NvmeTargetGetNqn(nodeConfig.Storage.DataNvme.Subsystem); err != nil {
	                        err = fmt.Errorf(errorFormat, err)
		                return
	                }
	                for i := range nodeConfig.Network.NvmeHost {
		                var lifs []string
		                if lifs, err = c.DiscoverNvmeLIFs(dataNvmeNamespacePath, nodeConfig.Network.NvmeHost[i].Subnet); err != nil {
	                                err = fmt.Errorf(errorFormat, err)
			                return
		                }
		                nodeConfig.Network.NvmeHost[i].NvmeTarget = &config.NvmeTarget{}
		                nodeConfig.Network.NvmeHost[i].NvmeTarget.TargetNqn = targetNqn
		                nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces = append(nodeConfig.Network.NvmeHost[i].NvmeTarget.Interfaces, lifs...)
                        }
	        }
        }
	return
}
