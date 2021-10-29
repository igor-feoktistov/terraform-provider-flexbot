package ontap

import (
	"fmt"
	"path/filepath"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

// CreateBootStorage creates node boot storage in cDOT
func CreateBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	imageLunPath := "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + nodeConfig.Storage.BootLun.OsImage.Name
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	var volumeExists bool
	if volumeExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	if !volumeExists {
		var aggregateName string
		if aggregateName, err = c.GetAggregateMax(nodeConfig); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		if err = c.VolumeCreateSAN(nodeConfig.Storage.VolumeName, aggregateName, (nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
	}
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	if !igroupExists {
		if c.IgroupCreate(nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		for i := range nodeConfig.Network.IscsiInitiator {
			if c.IgroupAddInitiator(nodeConfig.Storage.IgroupName, nodeConfig.Network.IscsiInitiator[i].InitiatorName); err != nil {
				err = fmt.Errorf("CreateBootStorage(): %s", err)
				return
			}
		}
	}
	var lunExists bool
	if lunExists, err = c.LunExists(bootLunPath); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	if !lunExists {
		if err = c.LunCopy(imageLunPath, bootLunPath); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		if err = c.LunResize(bootLunPath, nodeConfig.Storage.BootLun.Size); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
	}
	var lunMapped bool
	if lunMapped, err = c.IsLunMapped(bootLunPath, nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	if !lunMapped {
		if err = c.LunMap(bootLunPath, 0, nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
	}
	if nodeConfig.Storage.DataLun.Size > 0 {
		if lunExists, err = c.LunExists(dataLunPath); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		if !lunExists {
			if err = c.LunCreate(dataLunPath, nodeConfig.Storage.DataLun.Size); err != nil {
				err = fmt.Errorf("CreateBootStorage(): %s", err)
				return
			}
		}
		if lunMapped, err = c.IsLunMapped(dataLunPath, nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		if !lunMapped {
			if err = c.LunMap(dataLunPath, nodeConfig.Storage.DataLun.Id, nodeConfig.Storage.IgroupName); err != nil {
				err = fmt.Errorf("CreateBootStorage(): %s", err)
				return
			}
		}
	}
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf("CreateBootStorage(): %s", err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("CreateBootStorage(): %s", err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	return
}

// CreateBootStoragePreflight is sanity check before actual storage provisioning
func CreateBootStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
		return
	}
	if _, err = c.GetAggregateMax(nodeConfig); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
		return
	}
	var images []string
	var repoLunPath string
	if images, err = GetRepoImages(nodeConfig); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
		return
	}
	for _, image := range images {
		if image == nodeConfig.Storage.BootLun.OsImage.Name {
			repoLunPath = "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + image
		}
	}
	if repoLunPath == "" {
		err = fmt.Errorf("CreateBootStoragePreflight(): image \"%s\" not found in image repository volume \"%s\"", nodeConfig.Storage.BootLun.OsImage.Name, nodeConfig.Storage.ImageRepoName)
		return
	}
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(repoLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	return
}

// DeleteBootStorage deletes node boot storage
func DeleteBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight(): %s", err)
		return
	}
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("DeleteBootStorage(): %s", err)
		return
	}
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.DataLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		if lunExists, err = c.LunExists(lunPath); err != nil {
			err = fmt.Errorf("DeleteBootStorage(): %s", err)
			return
		}
		if lunExists {
			if igroupExists {
				if err = c.LunUnmap(lunPath, nodeConfig.Storage.IgroupName); err != nil {
					err = fmt.Errorf("DeleteBootStorage(): %s", err)
					return
				}
			}
			if err = c.LunDestroy(lunPath); err != nil {
				err = fmt.Errorf("DeleteBootStorage(): %s", err)
				return
			}
		}
	}
	if igroupExists {
		if err = c.IgroupDestroy(nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf("DeleteBootStorage(): %s", err)
			return
		}
	}
	var volumeExists bool
	if volumeExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("DeleteBootStorage(): %s", err)
		return
	}
	if volumeExists {
		if err = c.VolumeDestroy(nodeConfig.Storage.VolumeName); err != nil {
			err = fmt.Errorf("DeleteBootStorage(): %s", err)
		}
	}
	return
}

// DeleteBootLUNs deletes LUN's preserving hosting volumes
func DeleteBootLUNs(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): %s", err)
		return
	}
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): %s", err)
		return
	}
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.DataLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		if lunExists, err = c.LunExists(lunPath); err != nil {
			err = fmt.Errorf("DeleteBootLUNs(): %s", err)
			return
		}
		if lunExists {
			if igroupExists {
				if err = c.LunUnmap(lunPath, nodeConfig.Storage.IgroupName); err != nil {
					err = fmt.Errorf("DeleteBootLUNs(): %s", err)
					return
				}
			}
			if err = c.LunDestroy(lunPath); err != nil {
				err = fmt.Errorf("DeleteBootLUNs(): %s", err)
				return
			}
		}
	}
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.VolumeName, "/seed"); err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): %s", err)
		return
	}
	if fileExists {
		if err = c.FileDelete(nodeConfig.Storage.VolumeName, "/seed"); err != nil {
			err = fmt.Errorf("DeleteBootLUNs(): %s", err)
		}
	}
	return
}

// DiscoverBootStorage discovers boot storage in cDOT
func DiscoverBootStorage(nodeConfig *config.NodeConfig) (storageExists bool, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	seedLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name
	if storageExists, err = c.VolumeExists(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	if !storageExists {
		return
	}
	var lunInfo *client.LunInfo
	if lunInfo, err = c.LunGetInfo(bootLunPath); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	if lunInfo.Comment != "" {
		nodeConfig.Storage.BootLun.OsImage.Name = lunInfo.Comment
	}
	nodeConfig.Storage.BootLun.Size = lunInfo.Size
	if lunInfo, err = c.LunGetInfo(dataLunPath); err == nil {
		nodeConfig.Storage.DataLun.Size = lunInfo.Size
	}
	if lunInfo, err = c.LunGetInfo(seedLunPath); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	if lunInfo.Comment != "" {
		nodeConfig.Storage.SeedLun.SeedTemplate.Location = lunInfo.Comment
		nodeConfig.Storage.SeedLun.SeedTemplate.Name = filepath.Base(lunInfo.Comment)
	}
	var iscsiNodeName string
	if iscsiNodeName, err = c.IscsiTargetGetName(); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	for i := range nodeConfig.Network.IscsiInitiator {
		var lifs []string
		if lifs, err = c.DiscoverIscsiLIFs(bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("DiscoverBootStorage(): %s", err)
			return
		}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeName
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lifs...)
	}
	if nodeConfig.Storage.Snapshots, err = c.SnapshotGetList(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): %s", err)
		return
	}
	return
}

// ResizeBootStorage resizes boot storage in cDOT
func ResizeBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("ResizeBootStorage(): %s", err)
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	var bootLunInfo, dataLunInfo *client.LunInfo
	if bootLunInfo, err = c.LunGetInfo(bootLunPath); err != nil {
		err = fmt.Errorf("ResizeBootStorage(): %s", err)
		return
	}
	if bootLunInfo.Size > nodeConfig.Storage.BootLun.Size {
		err = fmt.Errorf("ResizeBootStorage(): cannot shrink boot LUN to requested size %d", nodeConfig.Storage.BootLun.Size)
		return
	}
	if dataLunInfo, err = c.LunGetInfo(dataLunPath); err == nil {
		if dataLunInfo.Size > nodeConfig.Storage.DataLun.Size {
			err = fmt.Errorf("ResizeBootStorage(): cannot shrink data LUN to requested size %d", nodeConfig.Storage.DataLun.Size)
			return
		}
	} else {
		dataLunInfo = &client.LunInfo{}
	}
	if nodeConfig.Storage.BootLun.Size > bootLunInfo.Size || nodeConfig.Storage.DataLun.Size > dataLunInfo.Size {
		if err = c.VolumeResize(nodeConfig.Storage.VolumeName, (nodeConfig.Storage.DataLun.Size+nodeConfig.Storage.BootLun.Size)*2); err != nil {
			err = fmt.Errorf("ResizeBootStorage(): %s", err)
			return
		}
		if nodeConfig.Storage.BootLun.Size > bootLunInfo.Size {
			if err = c.LunResize(bootLunPath, nodeConfig.Storage.BootLun.Size); err != nil {
				err = fmt.Errorf("ResizeBootStorage(): %s", err)
				return
			}
		}
		if nodeConfig.Storage.DataLun.Size > dataLunInfo.Size {
			if err = c.LunResize(dataLunPath, nodeConfig.Storage.DataLun.Size); err != nil {
				err = fmt.Errorf("ResizeBootStorage(): %s", err)
				return
			}
		}
	}
	return
}

// LunRestoreMapping restores LUN mappings (usually after snapshot restore)
func LunRestoreMapping(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("LunRestoreMapping(): %s", err)
		return
	}
	var exists bool
	if exists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("LunRestoreMapping(): %s", err)
		return
	}
	if !exists {
		err = fmt.Errorf("LunRestoreMapping(): igroup \"%s\" not found", nodeConfig.Storage.IgroupName)
		return
	}
	for _, lun := range []config.Lun{nodeConfig.Storage.BootLun.Lun, nodeConfig.Storage.SeedLun.Lun, nodeConfig.Storage.DataLun} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lun.Name
		if exists, err = c.LunExists(lunPath); err != nil {
			err = fmt.Errorf("LunRestoreMapping(): %s", err)
			return
		}
		if exists {
			var mapped bool
			if mapped, err = c.IsLunMapped(lunPath, nodeConfig.Storage.IgroupName); err != nil {
				err = fmt.Errorf("LunRestoreMapping(): %s", err)
				return
			}
			if !mapped {
				if err = c.LunMap(lunPath, lun.Id, nodeConfig.Storage.IgroupName); err != nil {
					err = fmt.Errorf("LunRestoreMapping(): %s", err)
					return
				}
			}
		}
	}
	return
}
