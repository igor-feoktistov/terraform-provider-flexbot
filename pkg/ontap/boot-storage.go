package ontap

import (
	"fmt"
	"strconv"
	"path/filepath"
	"math"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
)

func CreateBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	imageLunPath := "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + nodeConfig.Storage.BootLun.OsImage.Name
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	var aggregateName string
	// Find aggregate with MAX space available
	aggrOptions := &ontap.VserverShowAggrGetOptions{
		MaxRecords: 1024,
		Vserver:    nodeConfig.Storage.SvmName,
	}
	aggrResponse, _, err := c.VserverShowAggrGetAPI(aggrOptions)
	if err != nil {
		err = fmt.Errorf("CreateBootStorage: VserverShowAggrGetAPI() failure: %s", err)
		return
	} else {
		if aggrResponse.Results.NumRecords > 0 {
			var maxAvailableSize int
			for _, aggr := range aggrResponse.Results.AggrAttributes {
				if aggr.AvailableSize > maxAvailableSize {
					aggregateName = aggr.AggregateName
					maxAvailableSize = aggr.AvailableSize
				}
			}
			if (nodeConfig.Storage.BootLun.Size*1024*1024*1024+nodeConfig.Storage.DataLun.Size*1024*1024*1024)*2 > maxAvailableSize {
				err = fmt.Errorf("CreateBootStorage: VserverShowAggrGetAPI(): no aggregates found for requested storage size %dGB", (nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2)
				return
			}
		} else {
			err = fmt.Errorf("CreateBootStorage: VserverShowAggrGetAPI(): no aggregates found for vserver %s", nodeConfig.Storage.SvmName)
			return
		}
	}
	var volumeExists bool
	volumeExists, err = util.VolumeExists(c, nodeConfig.Storage.VolumeName)
	if err != nil {
		err = fmt.Errorf("CreateBootStorage: VolumeExists() failure: %s", err)
		return
	}
	if !volumeExists {
		// Volume size is a double SUM of LUN sizes to accomodate snapshots overhead
		volOptions := &ontap.VolumeCreateOptions{
			VolumeType:              "rw",
			Volume:                  nodeConfig.Storage.VolumeName,
			Size:                    strconv.Itoa((nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2) + "g",
			ContainingAggregateName: aggregateName,
		}
		// Create boot volume
		if _, _, err = c.VolumeCreateAPI(volOptions); err != nil {
			err = fmt.Errorf("CreateBootStorage: VolumeCreateAPI() failure: %s", err)
			return
		}
	}
	var igroupExists bool
	igroupExists, err = util.IgroupExists(c, nodeConfig.Storage.IgroupName)
	if err != nil {
		err = fmt.Errorf("CreateBootStorage: IgroupExists() failure: %s", err)
		return
	}
	if !igroupExists {
		// Create initiator group
		if _, _, err = c.IgroupCreateAPI(nodeConfig.Storage.IgroupName, "iscsi", "linux", ""); err != nil {
			err = fmt.Errorf("CreateBootStorage: IgroupCreateAPI() failure: %s", err)
			return
		}
		// Add initiators to initiator group from initiator configurations
		for i, _ := range nodeConfig.Network.IscsiInitiator {
			if _, _, err = c.IgroupAddAPI(nodeConfig.Storage.IgroupName, nodeConfig.Network.IscsiInitiator[i].InitiatorName, false); err != nil {
				err = fmt.Errorf("CreateBootStorage: IgroupAddAPI() failure: %s", err)
				return
			}
		}
	}
	var lunExists bool
	lunExists, err = util.LunExists(c, bootLunPath)
	if err != nil {
		err = fmt.Errorf("CreateBootStorage: LunExists() failure: %s", err)
		return
	}
	if !lunExists {
		// Copy boot LUN from image repository
		if err = util.LunCopy(c, imageLunPath, bootLunPath); err != nil {
			err = fmt.Errorf("CreateBootStorage: LunCopy() failure: %s", err)
			return
		}
		// Resize boot LUN per requested size
		resizeLunOptions := &ontap.LunResizeOptions{
			Path: bootLunPath,
			Size: nodeConfig.Storage.BootLun.Size * 1024 * 1024 * 1024,
		}
		if _, _, err = c.LunResizeAPI(resizeLunOptions); err != nil {
			err = fmt.Errorf("CreateBootStorage: LunResizeAPI() failure: %s", err)
			return
		}
	}
	var lunMapped bool
	lunMapped, err = util.IsLunMapped(c, bootLunPath, nodeConfig.Storage.IgroupName)
	if err != nil {
		err = fmt.Errorf("CreateBootStorage: IsLunMapped() failure: %s", err)
		return
	}
	if !lunMapped {
		// Map boot LUN to initiator group with LUN ID 0
		bootLunMapOptions := &ontap.LunMapOptions{
			LunId:          0,
			InitiatorGroup: nodeConfig.Storage.IgroupName,
			Path:           bootLunPath,
		}
		if _, _, err = c.LunMapAPI(bootLunMapOptions); err != nil {
			err = fmt.Errorf("CreateBootStorage: LunMapAPI() failure: %s", err)
			return
		}
	}
	// Create data LUN if requested
	if nodeConfig.Storage.DataLun.Size > 0 {
		lunExists, err = util.LunExists(c, dataLunPath)
		if err != nil {
			err = fmt.Errorf("CreateBootStorage: LunExists() failure: %s", err)
			return
		}
		if !lunExists {
			dataLunOptions := &ontap.LunCreateBySizeOptions{
				Path:   dataLunPath,
				Size:   nodeConfig.Storage.DataLun.Size * 1024 * 1024 * 1024,
				OsType: "linux",
			}
			if _, _, err = c.LunCreateBySizeAPI(dataLunOptions); err != nil {
				err = fmt.Errorf("CreateBootStorage: LunCreateBySizeAPI() failure: %s", err)
				return
			}
		}
		lunMapped, err = util.IsLunMapped(c, dataLunPath, nodeConfig.Storage.IgroupName)
		if err != nil {
			err = fmt.Errorf("CreateBootStorage: IsLunMapped() failure: %s", err)
			return
		}
		if !lunMapped {
			dataLunMapOptions := &ontap.LunMapOptions{
				LunId:          nodeConfig.Storage.DataLun.Id,
				InitiatorGroup: nodeConfig.Storage.IgroupName,
				Path:           dataLunPath,
			}
			if _, _, err = c.LunMapAPI(dataLunMapOptions); err != nil {
				err = fmt.Errorf("CreateBootStorage: LunMapAPI() failure: %s", err)
				return
			}
		}
	}
	var iscsiNodeGetNameResponse *ontap.IscsiNodeGetNameResponse
	// Fetching iSCSI target node name
	if iscsiNodeGetNameResponse, _, err = c.IscsiNodeGetNameAPI(); err != nil {
		err = fmt.Errorf("CreateBootStorage: IscsiNodeGetNameAPI() failure: %s", err)
		return
	}
	var lifs []*ontap.NetInterfaceInfo
	// Discover iSCSI LIF's and add LIF's IP's to iSCSI initiator configuration
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeGetNameResponse.Results.NodeName
		if lifs, err = util.DiscoverIscsiLIFs(c, bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			return
		}
		if len(lifs) > 0 {
			for _, lif := range lifs {
				nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lif.Address)
			}
		} else {
			err = fmt.Errorf("CreateBootStorage: DiscoverIscsiLIFs(): no iSCSI LIF's found for fabric %s: %s", nodeConfig.Network.IscsiInitiator[i].Name, err)
			return
		}
	}
	return
}

func CreateBootStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	// Find aggregate with MAX space available
	aggrOptions := &ontap.VserverShowAggrGetOptions{
		MaxRecords: 1024,
		Vserver:    nodeConfig.Storage.SvmName,
	}
	aggrResponse, _, err := c.VserverShowAggrGetAPI(aggrOptions)
	if err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight: VserverShowAggrGetAPI() failure: %s", err)
		return
	} else {
		if aggrResponse.Results.NumRecords > 0 {
			var maxAvailableSize int
			for _, aggr := range aggrResponse.Results.AggrAttributes {
				if aggr.AvailableSize > maxAvailableSize {
					maxAvailableSize = aggr.AvailableSize
				}
			}
			if (nodeConfig.Storage.BootLun.Size*1024*1024*1024+nodeConfig.Storage.DataLun.Size*1024*1024*1024)*2 > maxAvailableSize {
				err = fmt.Errorf("CreateBootStoragePreflight: VserverShowAggrGetAPI(): no aggregates found for requested storage size %dGB", (nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2)
				return
			}
		} else {
			err = fmt.Errorf("CreateBootStoragePreflight: VserverShowAggrGetAPI(): no aggregates found for vserver %s", nodeConfig.Storage.SvmName)
			return
		}
	}
	var images []string
	var repoLunPath string
	images, err = GetRepoImages(nodeConfig)
	for _, image := range images {
		if image == nodeConfig.Storage.BootLun.OsImage.Name {
			repoLunPath = "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + image
		}
	}
	if repoLunPath == "" {
		err = fmt.Errorf("CreateBootStoragePreflight: image \"%s\" not found in image repository volume \"%s\"", nodeConfig.Storage.BootLun.OsImage.Name, nodeConfig.Storage.ImageRepoName)
		return
	}
	var iscsiNodeGetNameResponse *ontap.IscsiNodeGetNameResponse
	// Fetching iSCSI target node name
	if iscsiNodeGetNameResponse, _, err = c.IscsiNodeGetNameAPI(); err != nil {
		err = fmt.Errorf("CreateBootStoragePreflight: IscsiNodeGetNameAPI() failure: %s", err)
		return
	}
	var lifs []*ontap.NetInterfaceInfo
	// Discover iSCSI LIF's and add LIF's IP's to iSCSI initiator configuration
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeGetNameResponse.Results.NodeName
		if lifs, err = util.DiscoverIscsiLIFs(c, repoLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			err = fmt.Errorf("CreateBootStoragePreflight: DiscoverIscsiLIFs(): %s", err)
			return
		}
		if len(lifs) > 0 {
			for _, lif := range lifs {
				nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lif.Address)
			}
		} else {
			err = fmt.Errorf("CreateBootStoragePreflight: DiscoverIscsiLIFs(): no iSCSI LIF's found for fabric %s: %s", nodeConfig.Network.IscsiInitiator[i].Name, err)
			return
		}
	}
	return
}

func DeleteBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	var response *ontap.SingleResultResponse
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteBootStorage: CreateCdotClient() failure: %s", err)
		return
	}
	var igroupExists bool
	igroupExists, err = util.IgroupExists(c, nodeConfig.Storage.IgroupName)
	if err != nil {
		err = fmt.Errorf("DeleteBootStorage: IgroupExists() failure: %s", err)
		return
	}
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.DataLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		lunExists, err = util.LunExists(c, lunPath)
		if err != nil {
			err = fmt.Errorf("DeleteBootStorage: LunExists() failure: %s", err)
			return
		}
		if lunExists {
			if igroupExists {
				lunUnmapOptions := &ontap.LunUnmapOptions{
					InitiatorGroup: nodeConfig.Storage.IgroupName,
					Path:           lunPath,
				}
				if response, _, err = c.LunUnmapAPI(lunUnmapOptions); err != nil {
					if !(response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_VDISK || response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_LUNMAP) {
						err = fmt.Errorf("DeleteBootStorage: LunUnmapAPI() failure: %s", err)
						return
					}
				}
			}
			lunDestroyOptions := &ontap.LunDestroyOptions{
				Path: lunPath,
			}
			if response, _, err = c.LunDestroyAPI(lunDestroyOptions); err != nil {
				if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
					err = fmt.Errorf("DeleteBootStorage: LunDestroyAPI() failure: %s", err)
					return
				}
			}
		}
	}
	if igroupExists {
		if _, _, err = c.IgroupDestroyAPI(nodeConfig.Storage.IgroupName, false); err != nil {
			err = fmt.Errorf("DeleteBootStorage: IgroupDestroyAPI() failure: %s", err)
			return
		}
	}
	var volumeExists bool
	volumeExists, err = util.VolumeExists(c, nodeConfig.Storage.VolumeName)
	if err != nil {
		err = fmt.Errorf("DeleteBootStorage: VolumeExists() failure: %s", err)
		return
	}
	if volumeExists {
		if response, _, err = c.VolumeOfflineAPI(nodeConfig.Storage.VolumeName); err != nil {
			if response.Results.ErrorNo != ontap.EVOLUMEOFFLINE {
				err = fmt.Errorf("DeleteBootStorage: VolumeOfflineAPI() failure: %s", err)
				return
			}
		}
		if _, _, err = c.VolumeDestroyAPI(nodeConfig.Storage.VolumeName); err != nil {
			err = fmt.Errorf("DeleteBootStorage: VolumeDestroyAPI() failure: %s", err)
		}
	}
	return
}

func DeleteBootLUNs(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	var response *ontap.SingleResultResponse
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): CreateCdotClient() failure: %s", err)
		return
	}
	var igroupExists bool
	igroupExists, err = util.IgroupExists(c, nodeConfig.Storage.IgroupName)
	if err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): IgroupExists() failure: %s", err)
		return
	}
	for _, lunName := range []string{nodeConfig.Storage.BootLun.Name, nodeConfig.Storage.DataLun.Name, nodeConfig.Storage.SeedLun.Name} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lunName
		var lunExists bool
		lunExists, err = util.LunExists(c, lunPath)
		if err != nil {
			err = fmt.Errorf("DeleteBootLUNs(): LunExists() failure: %s", err)
			return
		}
		if lunExists {
			if igroupExists {
				lunUnmapOptions := &ontap.LunUnmapOptions{
					InitiatorGroup: nodeConfig.Storage.IgroupName,
					Path:           lunPath,
				}
				if response, _, err = c.LunUnmapAPI(lunUnmapOptions); err != nil {
					if !(response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_VDISK || response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_LUNMAP) {
						err = fmt.Errorf("DeleteBootLUNs(): LunUnmapAPI() failure: %s", err)
						return
					}
				}
			}
			lunDestroyOptions := &ontap.LunDestroyOptions{
				Path: lunPath,
			}
			if response, _, err = c.LunDestroyAPI(lunDestroyOptions); err != nil {
				if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
					err = fmt.Errorf("DeleteBootLUNs(): LunDestroyAPI() failure: %s", err)
					return
				}
			}
		}
	}
	var fileExists bool
	if fileExists, err = util.FileExists(c, "/vol/" + nodeConfig.Storage.VolumeName + "/seed"); err != nil {
		err = fmt.Errorf("DeleteBootLUNs(): FileExists() failure: %s", err)
		return
	}
	if fileExists {
		if _, _, err = c.FileDeleteFileAPI("/vol/" + nodeConfig.Storage.VolumeName + "/seed"); err != nil {
			err = fmt.Errorf("DeleteRepoImage: FileDeleteFileAPI() failure: %s", err)
		}
	}
	return
}

func DiscoverBootStorage(nodeConfig *config.NodeConfig) (storageExists bool, err error) {
	var c *ontap.Client
	var lunResponse *ontap.LunGetResponse
	var lunOptions *ontap.LunGetOptions
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	seedLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name
	storageExists, err = util.VolumeExists(c, nodeConfig.Storage.VolumeName)
	if err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): VolumeExists() failure: %s", err)
		return
	}
	if !storageExists {
		return
	}
	lunOptions = &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: bootLunPath,
			},
		},
	}
	lunResponse, _, err = c.LunGetAPI(lunOptions)
	if err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): LunGetAPI() failure: %s", err)
		return
	}
	if lunResponse.Results.NumRecords == 0 {
		err = fmt.Errorf("DiscoverBootStorage(): LunGetAPI() failure: boot LUN %s not found", bootLunPath)
		return
	}
    	if lunResponse.Results.AttributesList.LunAttributes[0].Comment != "" {
    		nodeConfig.Storage.BootLun.OsImage.Name = lunResponse.Results.AttributesList.LunAttributes[0].Comment
	}
	nodeConfig.Storage.BootLun.Size = int(math.Round(float64(lunResponse.Results.AttributesList.LunAttributes[0].Size)/1024/1024/1024))
	lunOptions = &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: dataLunPath,
			},
		},
	}
	lunResponse, _, err = c.LunGetAPI(lunOptions)
	if err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): LunGetAPI() failure: %s", err)
		return
	}
	if lunResponse.Results.NumRecords > 0 {
		nodeConfig.Storage.DataLun.Size = int(math.Round(float64(lunResponse.Results.AttributesList.LunAttributes[0].Size)/1024/1024/1024))
	}
	lunOptions = &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: seedLunPath,
			},
		},
	}
	lunResponse, _, err = c.LunGetAPI(lunOptions)
	if err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): LunGetAPI() failure: %s", err)
		return
	}	
	if lunResponse.Results.NumRecords == 0 {
		err = fmt.Errorf("DiscoverBootStorage(): LunGetAPI() failure: seed LUN %s not found", seedLunPath)
		return
	}
    	if lunResponse.Results.AttributesList.LunAttributes[0].Comment != "" {
                nodeConfig.Storage.SeedLun.SeedTemplate.Location = lunResponse.Results.AttributesList.LunAttributes[0].Comment
		nodeConfig.Storage.SeedLun.SeedTemplate.Name = filepath.Base(lunResponse.Results.AttributesList.LunAttributes[0].Comment)
	}
	var iscsiNodeGetNameResponse *ontap.IscsiNodeGetNameResponse
	// Fetching iSCSI target node name
	if iscsiNodeGetNameResponse, _, err = c.IscsiNodeGetNameAPI(); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): IscsiNodeGetNameAPI() failure: %s", err)
		return
	}
	var lifs []*ontap.NetInterfaceInfo
	// Discover iSCSI LIF's and add LIF's IP's to iSCSI initiator configuration
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = iscsiNodeGetNameResponse.Results.NodeName
		if lifs, err = util.DiscoverIscsiLIFs(c, bootLunPath, nodeConfig.Network.IscsiInitiator[i].Subnet); err != nil {
			return
		}
		if len(lifs) > 0 {
			for _, lif := range lifs {
				nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, lif.Address)
			}
		} else {
			err = fmt.Errorf("DiscoverBootStorage(): DiscoverIscsiLIFs(): no iSCSI LIF's found for fabric %s: %s", nodeConfig.Network.IscsiInitiator[i].Name, err)
			return
		}
	}
	// Discover snapshots
	var snapOptions *ontap.SnapshotListInfoOptions
	var snapResponse *ontap.SnapshotListInfoResponse
	snapOptions = &ontap.SnapshotListInfoOptions {
		Volume: nodeConfig.Storage.VolumeName,
	}
	if snapResponse, _, err = c.SnapshotListInfoAPI(snapOptions); err != nil {
		err = fmt.Errorf("DiscoverBootStorage(): SnapshotListInfoAPI() failure: %s", err)
		return
	}
	nodeConfig.Storage.Snapshots = []string{}
	for _, snapshot := range snapResponse.Results.Snapshots {
		nodeConfig.Storage.Snapshots = append(nodeConfig.Storage.Snapshots, snapshot.Name)
	}
	return
}

func ResizeBootStorage(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	var response *ontap.LunGetResponse
	var options *ontap.LunGetOptions
	var bootLunSize, dataLunSize int
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	dataLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.DataLun.Name
	options = &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: bootLunPath,
			},
		},
	}
	response, _, err = c.LunGetAPI(options)
	if err != nil {
		err = fmt.Errorf("ResizeBootStorage(): LunGetAPI() failure: %s", err)
		return
	}
	if response.Results.NumRecords == 0 {
		err = fmt.Errorf("ResizeBootStorage(): LunGetAPI() failure: boot LUN %s not found", bootLunPath)
		return
	}
	bootLunSize = int(math.Round(float64(response.Results.AttributesList.LunAttributes[0].Size)/1024/1024/1024))
	if bootLunSize > nodeConfig.Storage.BootLun.Size {
		err = fmt.Errorf("ResizeBootStorage(): cannot shrink boot LUN to requested size %d", nodeConfig.Storage.BootLun.Size)
		return
	}
	options = &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: dataLunPath,
			},
		},
	}
	response, _, err = c.LunGetAPI(options)
	if err != nil {
		err = fmt.Errorf("ResizeBootStorage(): LunGetAPI() failure: %s", err)
		return
	}
	if response.Results.NumRecords > 0 {
		dataLunSize = int(math.Round(float64(response.Results.AttributesList.LunAttributes[0].Size)/1024/1024/1024))
		if dataLunSize > nodeConfig.Storage.DataLun.Size {
			err = fmt.Errorf("ResizeBootStorage(): cannot shrink data LUN to requested size %d", nodeConfig.Storage.DataLun.Size)
			return
		}
	}
	if nodeConfig.Storage.BootLun.Size > bootLunSize || nodeConfig.Storage.DataLun.Size > dataLunSize {
		if _, _, err = c.VolumeSizeAPI(nodeConfig.Storage.VolumeName, strconv.Itoa((nodeConfig.Storage.DataLun.Size + nodeConfig.Storage.BootLun.Size) * 2) + "g"); err != nil {
			err = fmt.Errorf("ResizeBootStorage(): VolumeSizeAPI() failure: %s", err)
			return
		}
		if nodeConfig.Storage.BootLun.Size > bootLunSize {
			resizeLunOptions := &ontap.LunResizeOptions{
				Path: bootLunPath,
				Size: nodeConfig.Storage.BootLun.Size * 1024 * 1024 * 1024,
			}
			if _, _, err = c.LunResizeAPI(resizeLunOptions); err != nil {
				err = fmt.Errorf("ResizeBootStorage():: LunResizeAPI() failure: %s", err)
				return
			}
		}
		if nodeConfig.Storage.DataLun.Size > dataLunSize {
			resizeLunOptions := &ontap.LunResizeOptions{
				Path: dataLunPath,
				Size: nodeConfig.Storage.DataLun.Size * 1024 * 1024 * 1024,
			}
			if _, _, err = c.LunResizeAPI(resizeLunOptions); err != nil {
				err = fmt.Errorf("ResizeBootStorage():: LunResizeAPI() failure: %s", err)
				return
			}
		}
	}
	return
}

func LunRestoreMapping(nodeConfig *config.NodeConfig) (err error) {
	var c *ontap.Client
	var exists, mapped bool
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	exists, err = util.IgroupExists(c, nodeConfig.Storage.IgroupName)
	if err != nil {
		err = fmt.Errorf("LunRestoreMapping: IgroupExists() failure: %s", err)
		return
	}
	if !exists {
		err = fmt.Errorf("LunRestoreMapping: igroup %s not found", nodeConfig.Storage.IgroupName)
		return
	}
	for _, lun := range []config.Lun{nodeConfig.Storage.BootLun.Lun, nodeConfig.Storage.SeedLun.Lun, nodeConfig.Storage.DataLun} {
		lunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + lun.Name
		exists, err = util.LunExists(c, lunPath)
		if err != nil {
			err = fmt.Errorf("LunRestoreMapping: LunExists() failure: %s", err)
			return
		}
		if exists {
			mapped, err = util.IsLunMapped(c, lunPath, nodeConfig.Storage.IgroupName)
			if err != nil {
				err = fmt.Errorf("LunRestoreMapping: IsLunMapped() failure: %s", err)
				return
			}
			if !mapped {
				lunMapOptions := &ontap.LunMapOptions{
					LunId:          lun.Id,
					InitiatorGroup: nodeConfig.Storage.IgroupName,
					Path:           lunPath,
				}
				if _, _, err = c.LunMapAPI(lunMapOptions); err != nil {
					err = fmt.Errorf("LunRestoreMapping: LunMapAPI() failure: %s", err)
					return
				}
			}
		}
	}
	return
}
