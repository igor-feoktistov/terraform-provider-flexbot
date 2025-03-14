package ontap

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

// CreateHarvesterStorage creates Harvester node storage in cDOT
func CreateHarvesterStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	errorFormat := "CreateHarvesterStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	imageLunPath := "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + nodeConfig.Storage.BootstrapLun.OsImage.Name
	bootstrapLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootstrapLun.Name
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	var volumeExists bool
	if volumeExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !volumeExists {
		var aggregateName string
		if aggregateName, err = c.GetAggregateMax(nodeConfig); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		if err = c.VolumeCreateSAN(nodeConfig.Storage.VolumeName, aggregateName, nodeConfig.Storage.BootLun.Size * 2); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !igroupExists {
		if c.IgroupCreate(nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		for i := range nodeConfig.Network.IscsiInitiator {
			if c.IgroupAddInitiator(nodeConfig.Storage.IgroupName, nodeConfig.Network.IscsiInitiator[i].InitiatorName); err != nil {
				err = fmt.Errorf(errorFormat, err)
				return
			}
		}
	}
	var lunExists bool
	if lunExists, err = c.LunExists(imageLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunExists {
		err = fmt.Errorf(errorFormat, "image " + imageLunPath + " not found")
		return
	}
	var lunInfo *client.LunInfo
	if lunInfo, err = c.LunGetInfo(imageLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	nodeConfig.Storage.BootstrapLun.Size = lunInfo.Size * 2
	if lunExists, err = c.LunExists(bootstrapLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunExists {
		if err = c.LunCopy(imageLunPath, bootstrapLunPath); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		if err = c.LunResize(bootstrapLunPath, nodeConfig.Storage.BootstrapLun.Size); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	var lunMapped bool
	if lunMapped, err = c.IsLunMapped(bootstrapLunPath, nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunMapped {
		if err = c.LunMap(bootstrapLunPath, 0, nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	if lunExists, err = c.LunExists(bootLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunExists {
		if err = c.LunCreate(bootLunPath, nodeConfig.Storage.BootLun.Size); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	if lunMapped, err = c.IsLunMapped(bootLunPath, nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunMapped {
		if err = c.LunMap(bootLunPath, 1, nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	return
}

// CreateBootStoragePreflight is sanity check before actual storage provisioning
func CreateHarvesterStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	errorFormat := "CreateHarvesterStoragePreflight(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if _, err = c.GetAggregateMax(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	var images []string
	var repoLunPath string
	if images, err = GetRepoImages(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	for _, image := range images {
		if image == nodeConfig.Storage.BootLun.OsImage.Name {
			repoLunPath = "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + image
		}
	}
	if repoLunPath == "" {
		err = fmt.Errorf("CreateHarvesterStoragePreflight(): image \"%s\" not found in image repository volume \"%s\"", nodeConfig.Storage.BootLun.OsImage.Name, nodeConfig.Storage.ImageRepoName)
		return
	}
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(repoLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	return
}

// DeleteHarvesterStorage deletes Harvester node storage
func DeleteHarvesterStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	errorFormat := "DeleteHarvesterStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.BootstrapLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		if lunExists, err = c.LunExists(lunPath); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		if lunExists {
			if igroupExists {
				if err = c.LunUnmap(lunPath, nodeConfig.Storage.IgroupName); err != nil {
					err = fmt.Errorf(errorFormat, err)
					return
				}
			}
			if err = c.LunDestroy(lunPath); err != nil {
				err = fmt.Errorf(errorFormat, err)
				return
			}
		}
	}
	if igroupExists {
		if err = c.IgroupDestroy(nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	var volumeExists bool
	if volumeExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if volumeExists {
                if err = DeleteNvmeStorage(nodeConfig); err != nil {
		        err = fmt.Errorf(errorFormat, err)
		        return
                }
		if err = c.VolumeDestroy(nodeConfig.Storage.VolumeName); err != nil {
			err = fmt.Errorf(errorFormat, err)
		}
	}
	return
}

// RemapHarvesterStorage re-maps Harvester node storage
func RemapHarvesterStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	var lunPath string
	var igroupExists, lunExists bool
	errorFormat := "RemapHarvesterStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.BootstrapLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath = "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		if lunExists, err = c.LunExists(lunPath); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		if lunExists {
			if igroupExists {
				if err = c.LunUnmap(lunPath, nodeConfig.Storage.IgroupName); err != nil {
					err = fmt.Errorf(errorFormat, err)
					return
				}
			}
			if lunName != nodeConfig.Storage.BootLun.Name {
				if err = c.LunDestroy(lunPath); err != nil {
					err = fmt.Errorf(errorFormat, err)
					return
				}
			}
		}
	}
	if lunExists, err = c.LunExists(bootLunPath); err == nil {
		if lunExists {
			if err = c.LunMap(bootLunPath, 0, nodeConfig.Storage.IgroupName); err != nil {
				err = fmt.Errorf(errorFormat, err)
			}
		} else {
			err = fmt.Errorf(errorFormat, "missing boot LUN " + bootLunPath)
		}
	}
	return
}

// DiscoverHarvesterStorage discovers Harvester storage in cDOT
func DiscoverHarvesterStorage(nodeConfig *config.NodeConfig) (storageExists bool, err error) {
	var c client.OntapClient
	errorFormat := "DiscoverHarvesterStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	if storageExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !storageExists {
		return
	}
	if storageExists, err = c.LunExists(bootLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !storageExists {
		return
	}
	var lunInfo *client.LunInfo
	if lunInfo, err = c.LunGetInfo(bootLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if lunInfo.Comment != "" {
		nodeConfig.Storage.BootLun.OsImage.Name = lunInfo.Comment
	}
	nodeConfig.Storage.BootLun.Size = lunInfo.Size
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	if nodeConfig.Storage.Snapshots, err = c.SnapshotGetList(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf(errorFormat, err)
	}
	return
}
