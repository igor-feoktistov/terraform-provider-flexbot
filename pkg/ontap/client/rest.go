package client

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"time"

	"github.com/igor-feoktistov/go-ontap-rest/ontap"
	"github.com/igor-feoktistov/go-ontap-rest/util"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

const (
        MAX_WAIT_FOR_LUN = 300
        LUN_SIZE_BASE = 1024 * 1024
        LUN_SIZE_OVERHEAD = 1024 * 1024
)

// OntapRestAPI is ontap REST API client
type OntapRestAPI struct {
	Client *ontap.Client
	Svm    string
}

// NewOntapRestAPI creates REST API client
func NewOntapRestAPI(nodeConfig *config.NodeConfig) (c *OntapRestAPI, err error) {
	c = &OntapRestAPI{}
	c.Client = ontap.NewClient(
		"https://"+nodeConfig.Storage.CdotCredentials.Host,
		&ontap.ClientOptions{
			BasicAuthUser:     nodeConfig.Storage.CdotCredentials.User,
			BasicAuthPassword: nodeConfig.Storage.CdotCredentials.Password,
			SSLVerify:         false,
			Debug:             false,
			Timeout:           60 * time.Second,
		},
	)
	var svms []ontap.Svm
	if svms, _, err = c.Client.SvmGetIter([]string{}); err != nil {
		err = fmt.Errorf("SvmGetIter(() failure: %s", err)
		return
	}
	if len(svms) > 0 {
		nodeConfig.Storage.SvmName = svms[0].Name
		c.Svm = svms[0].Name
	} else {
		err = fmt.Errorf("SvmGetIter(): failure: unexpected result, no SVMs returned")
	}
	return
}

// GetAggregateMax finds aggregate with maximum size available
func (c *OntapRestAPI) GetAggregateMax(nodeConfig *config.NodeConfig) (aggregateName string, err error) {
	var spaceAvailable int64
	if aggregateName, spaceAvailable, err = util.GetAggregateMax(c.Client, c.Svm); err != nil {
		err = fmt.Errorf("GetAggregateMax() failure: %s", err)
	} else {
		if (int64(nodeConfig.Storage.BootLun.Size)*1024*1024*1024+int64(nodeConfig.Storage.DataLun.Size)*1024*1024*1024)*2 > spaceAvailable {
			err = fmt.Errorf("GetAggregateMax(): no aggregates found for requested storage size %dGB", (nodeConfig.Storage.BootLun.Size+nodeConfig.Storage.DataLun.Size)*2)
		}
	}
	return
}

// VolumeExists checks if volume exists
func (c *OntapRestAPI) VolumeExists(volumeName string) (exists bool, err error) {
	var volumes []ontap.Volume
	if volumes, _, err = c.Client.VolumeGetIter([]string{"name=" + volumeName}); err != nil {
		err = fmt.Errorf("VolumeGetIter() failure: %s", err)
	} else {
		if len(volumes) > 0 {
			exists = true
		} else {
			exists = false
		}
	}
	return
}

// VolumeGet gets volume attributes
func (c *OntapRestAPI) VolumeGet(volumeName string) (volume *ontap.Volume, res *ontap.RestResponse, err error) {
	var volumes []ontap.Volume
	if volumes, res, err = c.Client.VolumeGetIter([]string{"name=" + volumeName}); err != nil {
		err = fmt.Errorf("VolumeGetIter() failure: %s", err)
		return
	}
	if len(volumes) > 0 {
		volume = &volumes[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("VolumeGet() failure: volume \"%s\" not found", volumeName)
	}
	return
}

// VolumeCreateSAN creates volume for SAN
func (c *OntapRestAPI) VolumeCreateSAN(volumeName string, aggregateName string, volumeSize int) (err error) {
	sizeBytes := volumeSize * 1024 * 1024 * 1024
	snapReservePct := 0
	volume := ontap.Volume{
		Resource: ontap.Resource{
			Name: volumeName,
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		Aggregates: []ontap.Resource{
			ontap.Resource{
				Name: aggregateName,
			},
		},
		Guarantee: &ontap.VolumeSpaceGuarantee{
			Type: "none",
		},
		Size: &sizeBytes,
		Space: &ontap.VolumeSpace{
			Snapshot: &ontap.VolumeSnapshotSettigs{
				ReservePercent: &snapReservePct,
			},
		},
		SnapshotPolicy: &ontap.Resource{
		        Name: "none",
		},
	}
	if _, err = c.Client.VolumeCreate(&volume, []string{}); err != nil {
		err = fmt.Errorf("VolumeCreate() failure: %s", err)
	}
	return
}

// VolumeCreateNAS creates volume for NAS
func (c *OntapRestAPI) VolumeCreateNAS(volumeName string, aggregateName string, exportPolicyName string, volumeSize int) (err error) {
	sizeBytes := volumeSize * 1024 * 1024 * 1024
	volume := ontap.Volume{
		Resource: ontap.Resource{
			Name: volumeName,
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		Aggregates: []ontap.Resource{
			ontap.Resource{
				Name: aggregateName,
			},
		},
		Guarantee: &ontap.VolumeSpaceGuarantee{
			Type: "none",
		},
		Nas: &ontap.Nas{
			ExportPolicy: &ontap.ExportPolicyRef{
				Resource: ontap.Resource{
					Name: exportPolicyName,
				},
			},
			Path: "/" + volumeName,
		},
		Size: &sizeBytes,
	}
	if _, err = c.Client.VolumeCreate(&volume, []string{}); err != nil {
		err = fmt.Errorf("VolumeCreate() failure: %s", err)
	}
	return
}

// VolumeDestroy deletes volume
func (c *OntapRestAPI) VolumeDestroy(volumeName string) (err error) {
	var volume *ontap.Volume
	var res *ontap.RestResponse
	if volume, res, err = c.VolumeGet(volumeName); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("VolumeDestroy(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.VolumeDelete(volume.GetRef(), []string{}); err != nil {
		err = fmt.Errorf("VolumeDelete() failure: %s", err)
	}
	return
}

// VolumeResize sets volume new size
func (c *OntapRestAPI) VolumeResize(volumeName string, volumeSize int) (err error) {
	sizeBytes := volumeSize * 1024 * 1024 * 1024
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	volumeResized := ontap.Volume{
		Size: &sizeBytes,
	}
	if _, err = c.Client.VolumeModify(volume.GetRef(), &volumeResized, []string{}); err != nil {
		err = fmt.Errorf("VolumeModify() failure: %s", err)
	}
	return
}

// ExportPolicyCreate creates export-policy
func (c *OntapRestAPI) ExportPolicyCreate(exportPolicyName string) (err error) {
	exportPolicy := ontap.ExportPolicy{
		ExportPolicyRef: ontap.ExportPolicyRef{
			Resource: ontap.Resource{
				Name: exportPolicyName,
			},
		},
	}
	if _, err = c.Client.ExportPolicyCreate(&exportPolicy, []string{}); err != nil {
		err = fmt.Errorf("ExportPolicyCreate() failure: %s", err)
	}
	return
}

// IgroupExists checks if iGroup exists
func (c *OntapRestAPI) IgroupExists(igroupName string) (exists bool, err error) {
	var igroups []ontap.Igroup
	if igroups, _, err = c.Client.IgroupGetIter([]string{"name=" + igroupName}); err != nil {
		err = fmt.Errorf("IgroupGetIter() failure: %s", err)
	} else {
		if len(igroups) > 0 {
			exists = true
		} else {
			exists = false
		}
	}
	return
}

// IgroupGet gets iGroup attributes
func (c *OntapRestAPI) IgroupGet(igroupName string) (igroup *ontap.Igroup, res *ontap.RestResponse, err error) {
	var igroups []ontap.Igroup
	if igroups, res, err = c.Client.IgroupGetIter([]string{"name=" + igroupName}); err != nil {
		err = fmt.Errorf("IgroupGetIter() failure: %s", err)
		return
	}
	if len(igroups) > 0 {
		igroup = &igroups[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("IgroupGet() failure: igroup \"%s\" not found", igroupName)
	}
	return
}

// IgroupCreate creates iGroup
func (c *OntapRestAPI) IgroupCreate(igroupName string) (err error) {
	igroup := ontap.Igroup{
		Resource: ontap.Resource{
			Name: igroupName,
		},
		OsType:   "linux",
		Protocol: "iscsi",
	}
	if _, err = c.Client.IgroupCreate(&igroup, []string{}); err != nil {
		err = fmt.Errorf("IgroupCreate() failure: %s", err)
	}
	return
}

// IgroupAddInitiator adds iSCSI initiator to iGroup
func (c *OntapRestAPI) IgroupAddInitiator(igroupName string, initiatorName string) (err error) {
	var igroup *ontap.Igroup
	if igroup, _, err = c.IgroupGet(igroupName); err != nil {
		return
	}
	initiator := ontap.IgroupInitiator{
		IgroupInitiators: &[]ontap.Resource{
			ontap.Resource{
				Name: initiatorName,
			},
		},
	}
	if _, err = c.Client.IgroupInitiatorCreate(igroup.GetRef(), &initiator); err != nil {
		err = fmt.Errorf("IgroupInitiatorCreate() failure: %s", err)
	}
	return
}

// IgroupDestroy deletes iGroup
func (c *OntapRestAPI) IgroupDestroy(igroupName string) (err error) {
	var igroup *ontap.Igroup
	var res *ontap.RestResponse
	if igroup, res, err = c.IgroupGet(igroupName); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("IgroupDestroy(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.IgroupDelete(igroup.GetRef()); err != nil {
		err = fmt.Errorf("IgroupDelete() failure: %s", err)
	}
	return
}

// LunExists checks if LUN exists
func (c *OntapRestAPI) LunExists(lunPath string) (exists bool, err error) {
	var luns []ontap.Lun
	if luns, _, err = c.Client.LunGetIter([]string{"name=" + lunPath}); err != nil {
		err = fmt.Errorf("LunGetIter() failure: %s", err)
	} else {
		if len(luns) > 0 {
			exists = true
		} else {
			exists = false
		}
	}
	return
}

// LunGet gets LUN attributes
func (c *OntapRestAPI) LunGet(lunPath string) (lun *ontap.Lun, res *ontap.RestResponse, err error) {
	var luns []ontap.Lun
	if luns, res, err = c.Client.LunGetIter([]string{"name=" + lunPath, "fields=comment,space"}); err != nil {
		err = fmt.Errorf("LunGetIter() failure: %s", err)
		return
	}
	if len(luns) > 0 {
		lun = &luns[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("LunGet() failure: LUN \"%s\" not found", lunPath)
	}
	return
}

// IsLunMapped checks if LUN is mapped
func (c *OntapRestAPI) IsLunMapped(lunPath string, igroupName string) (mapped bool, err error) {
	var lunMaps []ontap.LunMap
	if lunMaps, _, err = c.Client.LunMapGetIter([]string{"lun.name=" + lunPath, "igroup.name=" + igroupName}); err != nil {
		err = fmt.Errorf("LunMapGetIter() failure: %s", err)
	} else {
		if len(lunMaps) > 0 {
			mapped = true
		} else {
			mapped = false
		}
	}
	return
}

// LunCopy copies LUN from src to dst
func (c *OntapRestAPI) LunCopy(lunSrcPath string, lunDstPath string) (err error) {
	volumeDstName := filepath.Base(filepath.Dir(lunDstPath))
	lunDstName := filepath.Base(lunDstPath)
	req := ontap.Lun{
		Resource: ontap.Resource{
			Name: lunDstPath,
		},
		Location: &ontap.LunLocation{
			LogicalUnit: lunDstName,
			Volume: &ontap.Resource{
				Name: volumeDstName,
			},
		},
		Copy: &ontap.LunCopy{
		        Source: ontap.NameReference{
		                Name: lunSrcPath,
		        },
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
	}
	if _, _, err = c.Client.LunCreate(&req, []string{}); err != nil {
		err = fmt.Errorf("LunCopy() failure, src LUN %s, dst LUN %s: %s", lunSrcPath, lunDstPath, err)
	        return
	}
	giveupTime := time.Now().Add(time.Second * MAX_WAIT_FOR_LUN)
	for time.Now().Before(giveupTime) {
		var lun *ontap.Lun
		if lun, _, err = c.Client.LunGetByPath(lunDstPath, []string{"fields=status"}); err != nil {
		        err = fmt.Errorf("LunCopy() failure to get status for LUN %s: %s", lunDstPath, err)
			break
		}
		if lun.Status.State == "online" {
			return
		}
		time.Sleep(time.Second)
	}
	if err == nil {
		err = fmt.Errorf("LunCopy(): destination LUN is not available, maximum wait time exceeded")
	}
	return
}

// LunResize sets LUN new size
func (c *OntapRestAPI) LunResize(lunPath string, lunSize int) (err error) {
	var lun *ontap.Lun
	if lun, _, err = c.LunGet(lunPath); err != nil {
		return
	}
	sizeBytes := int64(lunSize) * 1024 * 1024 * 1024
	lunResized := ontap.Lun{
		Space: &ontap.LunSpace{
			Size: &sizeBytes,
		},
	}
	if _, err = c.Client.LunModify(lun.GetRef(), &lunResized); err != nil {
		err = fmt.Errorf("LunModify() failure: %s", err)
	}
	return
}

// LunMap maps LUN to iGroup
func (c *OntapRestAPI) LunMap(lunPath string, lunID int, igroupName string) (err error) {
	lunMap := ontap.LunMap{
		Igroup: &ontap.IgroupRef{
			Resource: ontap.Resource{
				Name: igroupName,
			},
		},
		Lun: &ontap.LunRef{
			Resource: ontap.Resource{
				Name: lunPath,
			},
		},
		LogicalUnitNumber: &lunID,
	}
	if _, err = c.Client.LunMapCreate(&lunMap, []string{}); err != nil {
		err = fmt.Errorf("LunMapCreate() failure: %s", err)
	}
	return
}

// LunUnmap unmaps LUN from iGroup
func (c *OntapRestAPI) LunUnmap(lunPath string, igroupName string) (err error) {
	var lunMaps []ontap.LunMap
	if lunMaps, _, err = c.Client.LunMapGetIter([]string{"lun.name=" + lunPath, "igroup.name=" + igroupName}); err != nil {
		err = fmt.Errorf("LunMapGetIter() failure: %s", err)
	} else {
		if len(lunMaps) > 0 {
			if _, err = c.Client.LunMapDelete(lunMaps[0].Lun.Uuid, lunMaps[0].Igroup.Uuid); err != nil {
				err = fmt.Errorf("LunMapDelete() failure: %s", err)
			}
		}
	}
	return
}

// LunCreate creates LUN
func (c *OntapRestAPI) LunCreate(lunPath string, lunSize int) (err error) {
	volumeName := filepath.Base(filepath.Dir(lunPath))
	lunName := filepath.Base(lunPath)
	sizeBytes := int64(lunSize) * 1024 * 1024 * 1024
	lun := ontap.Lun{
		Resource: ontap.Resource{
			Name: lunPath,
		},
		Location: &ontap.LunLocation{
			LogicalUnit: lunName,
			Volume: &ontap.Resource{
				Name: volumeName,
			},
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		OsType: "linux",
		Space: &ontap.LunSpace{
			Size: &sizeBytes,
		},
	}
	if _, _, err = c.Client.LunCreate(&lun, []string{}); err != nil {
		err = fmt.Errorf("LunCreate() failure: %s", err)
	}
	return
}

// LunCreateFromFile creates LUN from file
func (c *OntapRestAPI) LunCreateFromFile(volumeName string, filePath string, lunPath string, lunComment string) (err error) {
	if err = util.LunCreateFromFile(c.Client, lunPath, "/vol/"+volumeName+filePath, "linux"); err != nil {
		err = fmt.Errorf("LunCreateFromFile() failure: %s", err)
	}
	return
}

// LunDestroy deletes LUN
func (c *OntapRestAPI) LunDestroy(lunPath string) (err error) {
	var lun *ontap.Lun
	var res *ontap.RestResponse
	if lun, res, err = c.LunGet(lunPath); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("LunDestroy(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.LunDelete(lun.GetRef()); err != nil {
		err = fmt.Errorf("LunDelete() failure: %s", err)
	}
	return
}

// LunGetInfo gets generic LUN attributes
func (c *OntapRestAPI) LunGetInfo(lunPath string) (lunInfo *LunInfo, err error) {
	var lun *ontap.Lun
	if lun, _, err = c.LunGet(lunPath); err != nil {
		return
	}
	lunInfo = &LunInfo{
		Comment: lun.Comment,
		Size:    int(math.Round(float64(*lun.Space.Size) / 1024 / 1024 / 1024)),
	}
	return
}

// LunGetList gets list of LUN's
func (c *OntapRestAPI) LunGetList(volumeName string) (lunList []string, err error) {
	lunList = []string{}
	var luns []ontap.Lun
	if luns, _, err = c.Client.LunGetIter([]string{"location.volume.name=" + volumeName, "fields=location"}); err != nil {
		err = fmt.Errorf("LunGetIter() failure: %s", err)
	} else {
		for _, lun := range luns {
			lunList = append(lunList, lun.Location.LogicalUnit)
		}
	}
	return
}

// IscsiTargetGetName gets target name
func (c *OntapRestAPI) IscsiTargetGetName() (targetName string, err error) {
	var iscsiServices []ontap.IscsiService
	if iscsiServices, _, err = c.Client.IscsiServiceGetIter([]string{"enabled=true", "fields=target"}); err != nil {
		err = fmt.Errorf("IscsiServiceGetIter() failure: %s", err)
	} else {
		if len(iscsiServices) > 0 {
			targetName = iscsiServices[0].Target.Name
		} else {
			err = fmt.Errorf("IscsiServiceGetIter() failure: iSCSI service is not running")
		}
	}
	return
}

// DiscoverIscsiLIFs get list of iSCSI interfaces for LUN
func (c *OntapRestAPI) DiscoverIscsiLIFs(lunPath string, initiatorSubnet string) (lifs []string, err error) {
	lifs = []string{}
	var ipInterfaces []ontap.IpInterface
	if ipInterfaces, err = util.DiscoverIscsiLIFs(c.Client, lunPath, initiatorSubnet); err != nil {
		err = fmt.Errorf("DiscoverIscsiLIFs() failure: %s", err)
		return
	}
	if len(ipInterfaces) == 0 {
		err = fmt.Errorf("DiscoverIscsiLIFs() no LIFs found for LUN \"%s\" and initiator subnet \"%s\"", lunPath, initiatorSubnet)
		return
	}
	for _, ipInterface := range ipInterfaces {
		lifs = append(lifs, ipInterface.Ip.Address)
	}
	return
}

// FileExists checks if file exists
func (c *OntapRestAPI) FileExists(volumeName string, filePath string) (exists bool, err error) {
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var files []ontap.FileInfo
	dirPath := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)
	if files, _, err = c.Client.FileGetIter(volume.Uuid, dirPath, []string{"type=file", "name=" + fileName}); err != nil {
		err = fmt.Errorf("FileGetIter(): failure: %s", err)
		return
	}
	if len(files) > 0 {
		exists = true
	} else {
		exists = false
	}
	return
}

// FileGetList gets list if files on specified volume
func (c *OntapRestAPI) FileGetList(volumeName string, dirPath string) (fileList []string, err error) {
	fileList = []string{}
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var files []ontap.FileInfo
	if files, _, err = c.Client.FileGetIter(volume.Uuid, dirPath, []string{"type=file"}); err != nil {
		err = fmt.Errorf("FileGetIter(): failure: %s", err)
		return
	}
	for _, file := range files {
		fileList = append(fileList, file.Name)
	}
	return
}

// FileDelete deletes file
func (c *OntapRestAPI) FileDelete(volumeName string, filePath string) (err error) {
	var volume *ontap.Volume
	var res *ontap.RestResponse
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	if res, err = c.Client.FileDelete(volume.Uuid, filePath, []string{}); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_NO_SUCH_FILE_OR_DIR {
			err = nil
		} else {
			err = fmt.Errorf("FileDelete(): failure: %s", err)
		}
	}
	return
}

// FileDownload gets file content
func (c *OntapRestAPI) FileDownload(volumeName string, filePath string) (fileContent []byte, err error) {
	if fileContent, err = util.DownloadFileAPI(c.Client, volumeName, filePath); err != nil {
		err = fmt.Errorf("DownloadFileAPI(): failure: %s", err)
	}
	return
}

// FileUploadAPI uploads file content via REST API
func (c *OntapRestAPI) FileUploadAPI(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileAPI(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileAPI(): failure: %s", err)
	}
	return
}

// FileUploadNFS uploads file content via NFS
func (c *OntapRestAPI) FileUploadNFS(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileNFS(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileNFS(): failure: %s", err)
	}
	return
}

// SnapshotGet gets snapshot attributes
func (c *OntapRestAPI) SnapshotGet(volumeName string, snapshotName string) (snapshot *ontap.Snapshot, res *ontap.RestResponse, err error) {
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var snapshots []ontap.Snapshot
	if snapshots, res, err = c.Client.SnapshotGetIter(volume.Uuid, []string{"name=" + snapshotName}); err != nil {
		err = fmt.Errorf("SnapshotGetIter(): failure: %s", err)
		return
	}
	if len(snapshots) > 0 {
		snapshot = &snapshots[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("SnapshotGetIter(): no snapshot \"%s\" found", snapshotName)
	}
	return
}

// SnapshotGetList gets list of snapshots
func (c *OntapRestAPI) SnapshotGetList(volumeName string) (snapshots []string, err error) {
	snapshots = []string{}
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var volumeSnapshots []ontap.Snapshot
	if volumeSnapshots, _, err = c.Client.SnapshotGetIter(volume.Uuid, []string{}); err != nil {
		err = fmt.Errorf("SnapshotGetIter(): failure: %s", err)
		return
	}
	for _, snapshot := range volumeSnapshots {
		snapshots = append(snapshots, snapshot.Name)
	}
	return
}

// SnapshotCreate creates snapshot
func (c *OntapRestAPI) SnapshotCreate(volumeName string, snapshotName string, snapshotComment string) (err error) {
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	snapshot := ontap.Snapshot{
		Resource: ontap.Resource{
			Name: snapshotName,
		},
		Comment: snapshotComment,
	}
	if _, err = c.Client.SnapshotCreate(volume.Uuid, &snapshot); err != nil {
		err = fmt.Errorf("SnapshotCreate(): failure: %s", err)
	}
	return
}

// SnapshotDelete deletes snapshot
func (c *OntapRestAPI) SnapshotDelete(volumeName string, snapshotName string) (err error) {
	var snapshot *ontap.Snapshot
	var res *ontap.RestResponse
	if snapshot, res, err = c.SnapshotGet(volumeName, snapshotName); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("SnapshotDelete(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.SnapshotDelete(snapshot.GetRef()); err != nil {
		err = fmt.Errorf("SnapshotDelete(): failure: %s", err)
	}
	return
}

// SnapshotRestore restores volume from snapshot
func (c *OntapRestAPI) SnapshotRestore(volumeName string, snapshotName string) (err error) {
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var snapshot *ontap.Snapshot
	if snapshot, _, err = c.SnapshotGet(volumeName, snapshotName); err != nil {
		return
	}
	if _, err = c.Client.VolumeModify(volume.GetRef(), &ontap.Volume{}, []string{"restore_to.snapshot.uuid=" + snapshot.Uuid}); err != nil {
		err = fmt.Errorf("VolumeModify(): failure: %s", err)
	}
	return
}

// Create LUN and upload data
func (c *OntapRestAPI) LunCreateAndUpload(volumeName string, filePath string, fileSize int64, fileReader io.Reader, lunPath string, lunComment string) (err error) {
        var sizeBytes, bytesWritten int64
	sizeBytes = fileSize + int64(LUN_SIZE_BASE + LUN_SIZE_OVERHEAD)
	lunName := filepath.Base(lunPath)
	lun := ontap.Lun{
	        Comment: lunComment,
		Resource: ontap.Resource{
			Name: lunPath,
		},
		Location: &ontap.LunLocation{
			LogicalUnit: lunName,
			Volume: &ontap.Resource{
				Name: volumeName,
			},
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		OsType: "linux",
		Space: &ontap.LunSpace{
			Size: &sizeBytes,
		},
	}
	var luns []ontap.Lun
	if luns, _, err = c.Client.LunCreate(&lun, []string{"return_records=true"}); err != nil {
		err = fmt.Errorf("LunCreateAndUpload(): LunCreate() failure: %s", err)
		return
        }
	if bytesWritten, _, err = c.Client.LunWrite(luns[0].GetRef(), 0, fileReader); err != nil {
		err = fmt.Errorf("LunCreateAndUpload(): LunWrite() failure: %s", err)
		return
	}
	if bytesWritten < fileSize {
		err = fmt.Errorf("LunCreateAndUpload(): LunWrite() short write: expected to write \"%d\" bytes, written \"%d\" bytes", fileSize, bytesWritten)
	}
	return
}

// NvmeSubsystemGet gets NVME Subsystem attributes
func (c *OntapRestAPI) NvmeSubsystemGet(subsystemName string) (subsystem *ontap.NvmeSubsystem, res *ontap.RestResponse, err error) {
	var subsystems []ontap.NvmeSubsystem
	if subsystems, res, err = c.Client.NvmeSubsystemGetIter([]string{"name=" + subsystemName}); err != nil {
		err = fmt.Errorf("NvmeSubsystemGetIter() failure: %s", err)
		return
	}
	if len(subsystems) > 0 {
		subsystem = &subsystems[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("NvmeSubsystemGet() failure: NVME Subsystem \"%s\" not found", subsystemName)
	}
	return
}

// Check if NVME Subsystem exists
func (c *OntapRestAPI) NvmeSubsystemExists(subsystemName string) (exists bool, err error) {
	var subsystems []ontap.NvmeSubsystem
	if subsystems, _, err = c.Client.NvmeSubsystemGetIter([]string{"name=" + subsystemName}); err != nil {
		err = fmt.Errorf("NvmeSubsystemGetIter() failure: %s", err)
	} else {
		if len(subsystems) > 0 {
			exists = true
		} else {
			exists = false
		}
	}
	return
}

// Create NVME Subsystem
func (c *OntapRestAPI) NvmeSubsystemCreate(subsystemName string) (err error) {
	deleteOnUnmap := false
	subsystem := ontap.NvmeSubsystem{
	        DeleteOnUnmap: &deleteOnUnmap,
		Name: subsystemName,
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		OsType: "linux",
	}
	if _, _, err := c.Client.NvmeSubsystemCreate(&subsystem, []string{}); err != nil {
		err = fmt.Errorf("NvmeSubsystemCreate() failure: %s", err)
        }
        return
}

// Destroy NVME Subsystem
func (c *OntapRestAPI) NvmeSubsystemDestroy(subsystemName string) (err error) {
	var subsystem *ontap.NvmeSubsystem
	var res *ontap.RestResponse
	if subsystem, res, err = c.NvmeSubsystemGet(subsystemName); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("NvmeSubsystemGet(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.NvmeSubsystemDelete(subsystem.GetRef(), []string{"allow_delete_with_hosts=true"}); err != nil {
		err = fmt.Errorf("NvmeSubsystemDestroy() failure: %s", err)
	}
	return
}

// Add Host to NVME Subsystem
func (c *OntapRestAPI) NvmeSubsystemAddHost(subsystemName string, hostNqn string) (err error) {
	var subsystem *ontap.NvmeSubsystem
	if subsystem, _, err = c.NvmeSubsystemGet(subsystemName); err != nil {
		return
	}
	host := ontap.NvmeHost{
	        Nqn: hostNqn,
	}
	if _, _, err := c.Client.NvmeHostCreate(subsystem.GetRef(), &host, []string{}); err != nil {
		err = fmt.Errorf("NvmeSubsystemAddHost() failure: %s", err)
	}
        return
}

// NvmeNamespaceGet gets NVME Namespace  attributes
func (c *OntapRestAPI) NvmeNamespaceGet(namespacePath string) (namespace *ontap.NvmeNamespace, res *ontap.RestResponse, err error) {
	var namespaces []ontap.NvmeNamespace
	if namespaces, res, err = c.Client.NvmeNamespaceGetIter([]string{"name=" + namespacePath, "fields=comment,space"}); err != nil {
		err = fmt.Errorf("NvmeNamespaceGetIter() failure: %s", err)
		return
	}
	if len(namespaces) > 0 {
		namespace = &namespaces[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("NvmeNamespaceGet() failure: NVME Namespace \"%s\" not found", namespacePath)
	}
	return
}

// NvmeNamespaceGetInfo gets generic NVME namespace attributes
func (c *OntapRestAPI) NvmeNamespaceGetInfo(namespacePath string) (namespaceInfo *NvmeNamespaceInfo, err error) {
	var namespace *ontap.NvmeNamespace
	if namespace, _, err = c.NvmeNamespaceGet(namespacePath); err != nil {
		return
	}
	namespaceInfo = &NvmeNamespaceInfo{
		Comment: namespace.Comment,
		Size:    int(math.Round(float64(*namespace.Space.Size) / 1024 / 1024 / 1024)),
	}
	return
}

// Check if NVME Namespace exists
func (c *OntapRestAPI) NvmeNamespaceExists(namespacePath string) (exists bool, err error) {
	var namespaces []ontap.NvmeNamespace
	if namespaces, _, err = c.Client.NvmeNamespaceGetIter([]string{"name=" + namespacePath}); err != nil {
		err = fmt.Errorf("NvmeNamespaceGetIter() failure: %s", err)
	} else {
		if len(namespaces) > 0 {
			exists = true
		} else {
			exists = false
		}
	}
        return
}

// Check if NVME Namespace is mapped to NVME Subsystem
func (c *OntapRestAPI) IsNvmeNamespaceMapped(namespacePath string) (mapped bool, err error) {
        var subsystemMapHref string
        mapped = false
	if subsystemMapHref, _, err = c.Client.NvmeSubsystemMapGetByPath(namespacePath); err != nil {
		err = fmt.Errorf("NvmeSubsystemMapGetByPath() failure: %s", err)
	} else {
	        if len(subsystemMapHref) > 0 {
	                mapped = true
	        }
	}
        return
}

// Resize NVME Namespace
func (c *OntapRestAPI) NvmeNamespaceResize(namespacePath string, namespaceSize int) (err error) {
	var namespace *ontap.NvmeNamespace
	if namespace, _, err = c.NvmeNamespaceGet(namespacePath); err != nil {
		return
	}
	sizeBytes := int64(namespaceSize) * 1024 * 1024 * 1024
	namespaceResized := ontap.NvmeNamespace{
		Space: &ontap.NvmeNamespaceSpace{
			Size: &sizeBytes,
		},
	}
	if _, err = c.Client.NvmeNamespaceModify(namespace.GetRef(), &namespaceResized); err != nil {
		err = fmt.Errorf("NvmeNamespaceModify() failure: %s", err)
	}
        return
}

// Map NVME Namespace to NVME Subsystem
func (c *OntapRestAPI) NvmeNamespaceMap(namespacePath string, subsystemName string) (err error) {
	subsystemMap := ontap.NvmeSubsystemMap{
	        Namespace: &ontap.Resource{
	                Name: namespacePath,
	        },
	        Subsystem: &ontap.Resource{
		        Name: subsystemName,
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
	}
	if _, _, err := c.Client.NvmeSubsystemMapCreate(&subsystemMap, []string{}); err != nil {
		err = fmt.Errorf("NvmeNamespaceMap() failure: %s", err)
	}
        return
}

// Remove NVME Namespace mapping to NVME Subsystem
func (c *OntapRestAPI) NvmeNamespaceUnmap(namespacePath string) (err error) {
	if subsystemMapHref, _, err := c.Client.NvmeSubsystemMapGetByPath(namespacePath); err != nil {
		err = fmt.Errorf("NvmeSubsystemMapGetByPath() failure: %s", err)
	} else {
	        if len(subsystemMapHref) > 0 {
	                if _, err := c.Client.NvmeSubsystemMapDelete(subsystemMapHref); err != nil {
		                err = fmt.Errorf("NvmeSubsystemMapDelete() failure: %s", err)
	                }
                }
        }
        return
}

// Create NVME Namespace
func (c *OntapRestAPI) NvmeNamespaceCreate(namespacePath string, namespaceSize int) (err error) {
	volumeName := filepath.Base(filepath.Dir(namespacePath))
	namespaceName := filepath.Base(namespacePath)
	sizeBytes := int64(namespaceSize) * 1024 * 1024 * 1024
	namespace := ontap.NvmeNamespace{
		Location: &ontap.NvmeNamespaceLocation{
		        Namespace: namespaceName,
			Volume: &ontap.Resource{
				Name: volumeName,
			},
		},
		Svm: &ontap.Resource{
			Name: c.Svm,
		},
		OsType: "linux",
		Space: &ontap.NvmeNamespaceSpace{
			Size: &sizeBytes,
		},
	}
	if _, _, err = c.Client.NvmeNamespaceCreate(&namespace, []string{}); err != nil {
		err = fmt.Errorf("NvmeNamespaceCreate() failure: %s", err)
	}
        return
}

// Destroy NVME Namespace
func (c *OntapRestAPI) NvmeNamespaceDestroy(namespacePath string) (err error) {
	var namespace *ontap.NvmeNamespace
	var res *ontap.RestResponse
	if namespace, res, err = c.NvmeNamespaceGet(namespacePath); err != nil {
		if res.ErrorResponse.Error.Code == ontap.ERROR_ENTRY_DOES_NOT_EXIST {
			err = nil
		} else {
			err = fmt.Errorf("NvmeNamespaceGet(): failure: %s", err)
		}
		return
	}
	if _, err = c.Client.NvmeNamespaceDelete(namespace.GetRef()); err != nil {
		err = fmt.Errorf("NvmeNamespaceDestroy() failure: %s", err)
	}
        return
}

// Retrieve NVME Subsystem target NQN
func (c *OntapRestAPI) NvmeTargetGetNqn(subsystemName string) (targetNqn string, err error) {
	var subsystem *ontap.NvmeSubsystem
	if subsystem, _, err = c.NvmeSubsystemGet(subsystemName); err == nil {
	        targetNqn = subsystem.TargetNqn
	}
        return
}

// GetNvmeLIFs get list of NVME interfaces
func (c *OntapRestAPI) GetNvmeLIFs() (lifs []string, err error) {
	lifs = []string{}
	var ipInterfaces []ontap.IpInterface
        if ipInterfaces, _, err = c.Client.IpInterfaceGetIter([]string{"fields=ip","enabled=true","state=up","services=data_nvme_tcp"}); err != nil {
                err = fmt.Errorf("GetNvmeLIFs() failure: %s", err)
    		return
    	}
    	if len(ipInterfaces) == 0 {
		err = fmt.Errorf("GetNvmeLIFs(): no IP interfaces found")
		return
    	}
	for _, ipInterface := range ipInterfaces {
		lifs = append(lifs, ipInterface.Ip.Address)
	}
	return
}

// DiscoverNvmeLIFs get list of NVME interfaces for NVME Namespace
func (c *OntapRestAPI) DiscoverNvmeLIFs(namespacePath string, hostSubnet string) (lifs []string, err error) {
	lifs = []string{}
	var ipInterfaces []ontap.IpInterface
	if ipInterfaces, err = util.DiscoverNvmeLIFs(c.Client, namespacePath, hostSubnet); err != nil {
		err = fmt.Errorf("DiscoverNvmeLIFs() failure: %s", err)
		return
	}
	if len(ipInterfaces) > 0 {
	        for _, ipInterface := range ipInterfaces {
		        lifs = append(lifs, ipInterface.Ip.Address)
	        }
	} else {
	        if lifs, err = c.GetNvmeLIFs(); err != nil {
	                return
	        }
	        if len(lifs) == 0 {
		        err = fmt.Errorf("DiscoverNvmeLIFs() no NVME LIFs found")
		}
	}
	return
}
