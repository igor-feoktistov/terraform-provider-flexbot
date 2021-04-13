package client

import (
	"fmt"
	"time"
	"io"
	"math"
	"path/filepath"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-rest/ontap"
	"github.com/igor-feoktistov/go-ontap-rest/util"
)

type OntapRestAPI struct {
	Client *ontap.Client
	Svm string
}

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

func (c *OntapRestAPI) GetAggregateMax(nodeConfig *config.NodeConfig) (aggregateName string, err error) {
	var spaceAvailable int
	if aggregateName, spaceAvailable, err = util.GetAggregateMax(c.Client); err != nil {
		err = fmt.Errorf("GetAggregateMax() failure: %s", err)
	} else {
		if (nodeConfig.Storage.BootLun.Size * 1024 * 1024 * 1024 + nodeConfig.Storage.DataLun.Size * 1024 * 1024 * 1024) * 2 > spaceAvailable {
			err = fmt.Errorf("GetAggregateMax(): no aggregates found for requested storage size %dGB", (nodeConfig.Storage.BootLun.Size + nodeConfig.Storage.DataLun.Size) * 2)
		}
        }
	return
}

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

func (c *OntapRestAPI) VolumeGet(volumeName string) (volume *ontap.Volume, res *ontap.RestResponse, err error) {
	var volumes []ontap.Volume
	if volumes, res, err = c.Client.VolumeGetIter([]string{"name=" + volumeName}); err != nil {
		err = fmt.Errorf("VolumeGetIter() failure: %s", err)
		return
	}
	if len(volumes) > 0 {
		volume =  &volumes[0]
	} else {
		res.ErrorResponse.Error.Code = ontap.ERROR_ENTRY_DOES_NOT_EXIST
		err = fmt.Errorf("VolumeGet() failure: volume \"%s\" not found", volumeName)
	}
	return
}

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
	}
	if _, err = c.Client.VolumeCreate(&volume, []string{}); err != nil {
		err = fmt.Errorf("VolumeCreate() failure: %s", err)
	}
	return
}

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

func (c *OntapRestAPI) IgroupCreate(igroupName string) (err error) {
	igroup := ontap.Igroup{
		Resource: ontap.Resource{
			Name: igroupName,
		},
		OsType: "linux",
		Protocol: "iscsi",
	}
	if _, err = c.Client.IgroupCreate(&igroup, []string{}); err != nil {
		err = fmt.Errorf("IgroupCreate() failure: %s", err)
	}
	return
}

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

func (c *OntapRestAPI) LunCopy(imagePath string, lunPath string) (err error) {
	if err = util.LunCopy(c.Client, imagePath, lunPath); err != nil {
		err = fmt.Errorf("LunCopy() failure: %s", err)
	}
	return
}

func (c *OntapRestAPI) LunResize(lunPath string, lunSize int) (err error) {
	var lun *ontap.Lun
	if lun, _, err = c.LunGet(lunPath); err != nil {
		return
	}
	sizeBytes := lunSize * 1024 * 1024 * 1024
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

func (c *OntapRestAPI) LunMap(lunPath string, lunId int, igroupName string) (err error) {
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
		LogicalUnitNumber: &lunId,
	}
	if _, err = c.Client.LunMapCreate(&lunMap, []string{}); err != nil {
		err = fmt.Errorf("LunMapCreate() failure: %s", err)
	}
	return
}

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

func (c *OntapRestAPI) LunCreate(lunPath string, lunSize int) (err error) {
        volumeName := filepath.Base(filepath.Dir(lunPath))
	lunName := filepath.Base(lunPath)
	sizeBytes := lunSize * 1024 * 1024 * 1024
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
	if _, err = c.Client.LunCreate(&lun, []string{}); err != nil {
		err = fmt.Errorf("LunCreate() failure: %s", err)
	}
	return
}

func (c *OntapRestAPI) LunCreateFromFile(volumeName string, filePath string, lunPath string, lunComment string) (err error) {
	if err = util.LunCreateFromFile(c.Client, lunPath, "/vol/" + volumeName + filePath, "linux"); err != nil {
		err = fmt.Errorf("LunCreateFromFile() failure: %s", err)
	}
	return
}

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

func (c *OntapRestAPI) LunGetInfo(lunPath string) (lunInfo *LunInfo, err error) {
	var lun *ontap.Lun
	if lun, _, err = c.LunGet(lunPath); err != nil {
		return
	}
    	lunInfo = &LunInfo{
    		Comment: lun.Comment,
    		Size: int(math.Round(float64(*lun.Space.Size)/1024/1024/1024)),
	}
	return
}

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

func (c *OntapRestAPI) IscsiTargetGetName() (targetName string, err error) {
	var iscsiServices []ontap.IscsiService
	if iscsiServices, _, err = c.Client.IscsiServiceGetIter([]string{"enabled=true","fields=target"}); err != nil {
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
		lifs  = append(lifs, ipInterface.Ip.Address)
	}
	return
}

func (c *OntapRestAPI) FileExists(volumeName string, filePath string) (exists bool, err error) {
	var volume *ontap.Volume
	if volume, _, err = c.VolumeGet(volumeName); err != nil {
		return
	}
	var files []ontap.FileInfo
	dirPath := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)
	if files, _, err = c.Client.FileGetIter(volume.Uuid, dirPath, []string{"type=file","name=" + fileName}); err != nil {
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

func (c *OntapRestAPI) FileDownload(volumeName string, filePath string) (fileContent []byte, err error) {
	if fileContent, err = util.DownloadFileAPI(c.Client, volumeName, filePath); err != nil {
		err = fmt.Errorf("DownloadFileAPI(): failure: %s", err)
	}
	return
}

func (c *OntapRestAPI) FileUploadAPI(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileAPI(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileAPI(): failure: %s", err)
	}
	return
}

func (c *OntapRestAPI) FileUploadNFS(volumeName string, filePath string, reader io.Reader) (err error) {
	if _, err = util.UploadFileNFS(c.Client, volumeName, filePath, reader); err != nil {
		err = fmt.Errorf("UploadFileNFS(): failure: %s", err)
	}
	return
}

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
		snapshots  = append(snapshots, snapshot.Name)
	}
	return
}

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
