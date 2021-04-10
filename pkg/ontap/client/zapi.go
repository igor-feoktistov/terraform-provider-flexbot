package client

import (
	"time"
	"fmt"
	"strconv"
	"strings"
	"math"
	"io"
	"encoding/hex"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
)

type OntapZAPI struct {
	Client *ontap.Client
}

func NewOntapZAPI(nodeConfig *config.NodeConfig) (c *OntapZAPI, err error) {
	c = &OntapZAPI{}
	c.Client = ontap.NewClient(
		"https://"+nodeConfig.Storage.CdotCredentials.Host,
		&ontap.ClientOptions{
			BasicAuthUser:     nodeConfig.Storage.CdotCredentials.User,
			BasicAuthPassword: nodeConfig.Storage.CdotCredentials.Password,
			SSLVerify:         false,
			Debug:             false,
			Timeout:           60 * time.Second,
			Version:           nodeConfig.Storage.CdotCredentials.ZapiVersion,
		},
	)
	var vserverOptions *ontap.VserverGetOptions
	if nodeConfig.Storage.SvmName == "" {
		// We don't need vserver name when connected to Vserver LIF
		vserverOptions = &ontap.VserverGetOptions{MaxRecords: 1}
	} else {
		// Name of vserver is required when connected to Cluster LIF
		vserverOptions = &ontap.VserverGetOptions{
			MaxRecords: 1,
			Query: &ontap.VserverInfo{
				VserverName: nodeConfig.Storage.SvmName,
			},
		}
	}
	var vserverResponse *ontap.VserverGetResponse
	if vserverResponse, _, err = c.Client.VserverGetAPI(vserverOptions); err != nil {
		return
	} else {
		if vserverResponse.Results.NumRecords == 1 {
			nodeConfig.Storage.SvmName = vserverResponse.Results.VserverAttributes[0].VserverName
			c.Client.SetVserver(nodeConfig.Storage.SvmName)
		} else {
			if nodeConfig.Storage.SvmName == "" {
				err = fmt.Errorf("CreateCdotClient(): expected svmName in storage configuration")
			} else {
				err = fmt.Errorf("CreateCdotClient: vserver not found: " + nodeConfig.Storage.SvmName)
			}
		}
	}
	return
}

func (c *OntapZAPI) GetAggregateMax(nodeConfig *config.NodeConfig) (aggregateName string, err error) {
	aggrOptions := &ontap.VserverShowAggrGetOptions{
		MaxRecords: 1024,
		Vserver:    nodeConfig.Storage.SvmName,
	}
	var aggrResponse *ontap.VserverShowAggrGetResponse
	if aggrResponse, _, err = c.Client.VserverShowAggrGetAPI(aggrOptions); err != nil {
		err = fmt.Errorf("VserverShowAggrGetAPI() failure: %s", err)
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
				err = fmt.Errorf("VserverShowAggrGetAPI(): no aggregates found for requested storage size %dGB", (nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2)
			}
		} else {
			err = fmt.Errorf("VserverShowAggrGetAPI(): no aggregates found for vserver %s", nodeConfig.Storage.SvmName)
		}
	}
	return
}

func (c *OntapZAPI) VolumeExists(volumeName string) (exists bool, err error) {
	exists, err = util.VolumeExists(c.Client, volumeName)
	return
}

func (c *OntapZAPI) VolumeCreateSAN(volumeName string, aggregateName string, volumeSize int) (err error) {
	volOptions := &ontap.VolumeCreateOptions{
		VolumeType:              "rw",
		Volume:                  volumeName,
		SpaceReserve:            "none",
		SnapshotPolicy:          "none",
		Size:                    strconv.Itoa(volumeSize) + "g",
		ContainingAggregateName: aggregateName,
	}
	if _, _, err = c.Client.VolumeCreateAPI(volOptions); err != nil {
		err = fmt.Errorf("VolumeCreateAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) VolumeCreateNAS(volumeName string, aggregateName string, exportPolicyName string, volumeSize int) (err error) {
	volOptions := &ontap.VolumeCreateOptions{
		VolumeType:              "rw",
		Volume:                  volumeName,
		JunctionPath:            "/" + volumeName,
		UnixPermissions:         "0755",
		Size:                    strconv.Itoa(volumeSize) + "g",
		ExportPolicy:            exportPolicyName,
		ContainingAggregateName: aggregateName,
	}
	if _, _, err = c.Client.VolumeCreateAPI(volOptions); err != nil {
		err = fmt.Errorf("CreateRepoImage: VolumeCreateAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) VolumeDestroy(volumeName string) (err error) {
	var response *ontap.SingleResultResponse
	if response, _, err = c.Client.VolumeOfflineAPI(volumeName); err != nil {
		if response.Results.ErrorNo != ontap.EVOLUMEOFFLINE {
			err = fmt.Errorf("VolumeOfflineAPI() failure: %s", err)
			return
		}
	}
	if _, _, err = c.Client.VolumeDestroyAPI(volumeName); err != nil {
		err = fmt.Errorf("VolumeDestroyAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) VolumeResize(volumeName string, volumeSize int) (err error) {
	if _, _, err = c.Client.VolumeSizeAPI(volumeName, strconv.Itoa(volumeSize) + "g"); err != nil {
		err = fmt.Errorf("VolumeSizeAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) ExportPolicyCreate(exportPolicyName string) (err error) {
	if _, _, err = c.Client.ExportPolicyCreateAPI(exportPolicyName, false); err != nil {
		err = fmt.Errorf("ExportPolicyCreateAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) IgroupExists(igroupName string) (exists bool, err error) {
	exists, err = util.IgroupExists(c.Client, igroupName)
	return
}

func (c *OntapZAPI) IgroupCreate(igroupName string) (err error) {
	if _, _, err = c.Client.IgroupCreateAPI(igroupName, "iscsi", "linux", ""); err != nil {
		err = fmt.Errorf("IgroupCreateAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) IgroupAddInitiator(igroupName string, initiatorName string) (err error) {
	if _, _, err = c.Client.IgroupAddAPI(igroupName, initiatorName, false); err != nil {
		err = fmt.Errorf("IgroupAddAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) IgroupDestroy(igroupName string) (err error) {
	if _, _, err = c.Client.IgroupDestroyAPI(igroupName, false); err != nil {
		err = fmt.Errorf("IgroupDestroyAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunExists(lunPath string) (exists bool, err error) {
	exists, err = util.LunExists(c.Client, lunPath)
	return
}

func (c *OntapZAPI) IsLunMapped(lunPath string, igroupName string) (mapped bool, err error) {
	mapped, err = util.IsLunMapped(c.Client, lunPath, igroupName)
	return
}

func (c *OntapZAPI) LunCopy(imagePath string, lunPath string) (err error) {
	if err = util.LunCopy(c.Client, imagePath, lunPath); err != nil {
		err = fmt.Errorf("LunCopy() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunResize(lunPath string, lunSize int) (err error) {
	resizeLunOptions := &ontap.LunResizeOptions{
		Path: lunPath,
		Size: lunSize * 1024 * 1024 * 1024,
	}
	if _, _, err = c.Client.LunResizeAPI(resizeLunOptions); err != nil {
		err = fmt.Errorf("LunResizeAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunMap(lunPath string, lunId int, igroupName string) (err error) {
	bootLunMapOptions := &ontap.LunMapOptions{
		LunId:          lunId,
		InitiatorGroup: igroupName,
		Path:           lunPath,
	}
	if _, _, err = c.Client.LunMapAPI(bootLunMapOptions); err != nil {
		err = fmt.Errorf("LunMapAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunUnmap(lunPath string, igroupName string) (err error) {
	lunUnmapOptions := &ontap.LunUnmapOptions{
		InitiatorGroup: igroupName,
		Path:           lunPath,
	}
	var response *ontap.SingleResultResponse
	if response, _, err = c.Client.LunUnmapAPI(lunUnmapOptions); err != nil {
		if !(response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_VDISK || response.Results.ErrorNo == ontap.EVDISK_ERROR_NO_SUCH_LUNMAP) {
			err = fmt.Errorf("LunUnmapAPI() failure: %s", err)
		} else {
			err = nil
		}
	}
	return
}

func (c *OntapZAPI) LunCreate(lunPath string, lunSize int) (err error) {
	lunOptions := &ontap.LunCreateBySizeOptions{
		Path:   lunPath,
		Size:   lunSize * 1024 * 1024 * 1024,
		OsType: "linux",
	}
	if _, _, err = c.Client.LunCreateBySizeAPI(lunOptions); err != nil {
		err = fmt.Errorf("LunCreateBySizeAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunCreateFromFile(volumeName string, filePath string, lunPath string, lunComment string) (err error) {
	lunCreateOptions := &ontap.LunCreateFromFileOptions{
		Comment:  lunComment,
		FileName: "/vol/" + volumeName + filePath,
		Path:     lunPath,
		OsType:   "linux",
	}
	if _, _, err = c.Client.LunCreateFromFileAPI(lunCreateOptions); err != nil {
		err = fmt.Errorf("LunCreateFromFileAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) LunDestroy(lunPath string) (err error) {
	lunDestroyOptions := &ontap.LunDestroyOptions{
		Path: lunPath,
	}
	var response *ontap.SingleResultResponse
	if response, _, err = c.Client.LunDestroyAPI(lunDestroyOptions); err != nil {
		if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
			err = fmt.Errorf("LunDestroyAPI() failure: %s", err)
		} else {
			err = nil
		}
	}
	return
}

func (c *OntapZAPI) LunGetInfo(lunPath string) (lunInfo *LunInfo, err error) {
	lunOptions := &ontap.LunGetOptions{
		MaxRecords: 1,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Path: lunPath,
			},
		},
	}
	var lunResponse *ontap.LunGetResponse
	if lunResponse, _, err = c.Client.LunGetAPI(lunOptions); err != nil {
		err = fmt.Errorf("LunGetAPI() failure: %s", err)
		return
	}
	if lunResponse.Results.NumRecords == 0 {
		err = fmt.Errorf("LunGetAPI() failure: LUN %s not found", lunPath)
		return
	}
    	lunInfo = &LunInfo{
    		Comment: lunResponse.Results.AttributesList.LunAttributes[0].Comment,
    		Size: int(math.Round(float64(lunResponse.Results.AttributesList.LunAttributes[0].Size)/1024/1024/1024)),
	}
	return
}

func (c *OntapZAPI) LunGetList(volumeName string) (lunList []string, err error) {
	lunList = []string{}
	options := &ontap.LunGetOptions{
		MaxRecords: 1024,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Volume: volumeName,
			},
		},
	}
	var response []*ontap.LunGetResponse
	response, err = c.Client.LunGetIterAPI(options)
	if err != nil {
		err = fmt.Errorf("LunGetIterAPI() failure: %s", err)
	} else {
		for _, responseLun := range response {
			for _, lun := range responseLun.Results.AttributesList.LunAttributes {
				lunList = append(lunList, lun.Path[(strings.LastIndex(lun.Path, "/")+1):])
			}
		}
	}
	return
}

func (c *OntapZAPI) IscsiTargetGetName() (targetName string, err error) {
	var iscsiNodeGetNameResponse *ontap.IscsiNodeGetNameResponse
	if iscsiNodeGetNameResponse, _, err = c.Client.IscsiNodeGetNameAPI(); err != nil {
		err = fmt.Errorf("IscsiNodeGetNameAPI() failure: %s", err)
	} else {
		targetName = iscsiNodeGetNameResponse.Results.NodeName
	}
	return
}

func (c *OntapZAPI) DiscoverIscsiLIFs(lunPath string, initiatorSubnet string) (lifs []string, err error) {
	var iscsiLifs []*ontap.NetInterfaceInfo
	lifs = []string{}
	if iscsiLifs, err = util.DiscoverIscsiLIFs(c.Client, lunPath, initiatorSubnet); err != nil {
		err = fmt.Errorf("DiscoverIscsiLIFs() failure: %s", err)
		return
	}
	if len(iscsiLifs) == 0 {
		err = fmt.Errorf("DiscoverIscsiLIFs() no LIFs found for LUN \"%s\" and initiator subnet \"%s\"", lunPath, initiatorSubnet)
		return
	}
	for _, lif := range iscsiLifs {
		lifs  = append(lifs, lif.Address)
	}
	return
}

func (c *OntapZAPI) FileExists(volumeName string, filePath string) (exists bool, err error) {
	if exists, err = util.FileExists(c.Client, "/vol/" + volumeName + filePath); err != nil {
		err = fmt.Errorf("FileExists() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) FileGetList(volumeName string, dirPath string) (fileList []string, err error) {
	listDirOptions := &ontap.FileListDirectoryOptions {
		    MaxRecords: 1024,
		    Path: "/vol/" + volumeName + dirPath,
	}
	var listDirResponse []*ontap.FileListDirectoryResponse
	if listDirResponse, err = c.Client.FileListDirectoryIterAPI(listDirOptions); err != nil {
		err = fmt.Errorf("FileListDirectoryIterAPI() failure: %s", err)
		return
	}
	for _, response := range listDirResponse {
		for _, fileAttr := range response.Results.AttributesList.FileAttributes {
			if !strings.HasPrefix(fileAttr.Name, ".") {
				fileList = append(fileList, fileAttr.Name)
			}
		}
	}
	return
}

func (c *OntapZAPI) FileDelete(volumeName string, filePath string) (err error) {
	if _, _, err = c.Client.FileDeleteFileAPI("/vol/" + volumeName + filePath); err != nil {
		err = fmt.Errorf("FileDeleteFileAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) FileDownload(volumeName string, filePath string) (fileContent []byte, err error) {
	var fileInfoResponse *ontap.FileGetFileInfoResponse
	if fileInfoResponse, _, err = c.Client.FileGetFileInfoAPI("/vol/" + volumeName + filePath); err != nil {
		err = fmt.Errorf("FileGetFileInfoAPI() failure: %s", err)
		return
	}
	readFileOptions := &ontap.FileReadFileOptions {
		Path: "/vol/" + volumeName + filePath,
		Offset: 0,
		Length: fileInfoResponse.Results.FileInfo.FileSize,
	}
	var readFileResponse *ontap.FileReadFileResponse
	if readFileResponse, _, err = c.Client.FileReadFileAPI(readFileOptions); err != nil {
		err = fmt.Errorf("FileReadFileAPI() failure: %s", err)
		return
	}
	bytesEncoded := []byte(readFileResponse.Results.Data)
	fileContent = make([]byte, hex.DecodedLen(len(bytesEncoded)))
	hex.Decode(fileContent, bytesEncoded)
	return
}

func (c *OntapZAPI) FileUploadAPI(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileAPI(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) FileUploadNFS(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileNFS(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileNFS() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) SnapshotGetList(volumeName string) (snapshots []string, err error) {
	snapshots = []string{}
	snapOptions := &ontap.SnapshotListInfoOptions {
		Volume: volumeName,
	}
	var snapResponse *ontap.SnapshotListInfoResponse
	if snapResponse, _, err = c.Client.SnapshotListInfoAPI(snapOptions); err != nil {
		err = fmt.Errorf("SnapshotListInfoAPI() failure: %s", err)
		return
	}
	for _, snapshot := range snapResponse.Results.Snapshots {
		snapshots = append(snapshots, snapshot.Name)
	}
	return
}

func (c *OntapZAPI) SnapshotCreate(volumeName string, snapshotName string, snapshotComment string) (err error) {
	options := &ontap.SnapshotCreateOptions {
		Volume: volumeName,
		Snapshot: snapshotName,
		Comment: snapshotComment,
	}
	if _, _, err = c.Client.SnapshotCreateAPI(options); err != nil {
		err = fmt.Errorf("SnapshotCreateAPI() failure: %s", err)
	}
	return
}

func (c *OntapZAPI) SnapshotDelete(volumeName string, snapshotName string) (err error) {
	options := &ontap.SnapshotDeleteOptions {
		Volume: volumeName,
		Snapshot: snapshotName,
	}
	var response *ontap.SingleResultResponse
	if response, _, err = c.Client.SnapshotDeleteAPI(options); err != nil {
		if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
			err = fmt.Errorf("SnapshotDeleteAPI() failure: %s", err)
		} else {
			err = nil
		}
	}
	return
}

func (c *OntapZAPI) SnapshotRestore(volumeName string, snapshotName string) (err error) {
	options := &ontap.SnapshotRestoreVolumeOptions {
		PreserveLunIds: false,
		Volume: volumeName,
		Snapshot: snapshotName,
	}
	if _, _, err = c.Client.SnapshotRestoreVolumeAPI(options); err != nil {
		err = fmt.Errorf("SnapshotRestoreVolumeAPI() failure: %s", err)
	}
	return
}
