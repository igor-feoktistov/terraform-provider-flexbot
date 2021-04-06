package client

import (
	"fmt"
	"io"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

type OntapClient interface {
	GetAggregateMax(nodeConfig *config.NodeConfig) (string, error)
	VolumeExists(volumeName string) (bool, error)
	VolumeCreateSAN(volumeName string, aggregateName string, volumeSize int) (error)
	VolumeCreateNAS(volumeName string, aggregateName string, exportPolicyName string, volumeSize int) (error)
	VolumeDestroy(volumeName string) (error)
	VolumeResize(volumeName string, volumeSize int) (error)
	ExportPolicyCreate(exportPolicyName string) (error)
	IgroupExists(volumeName string) (bool, error)
	IgroupCreate(igroupName string) (error) 
	IgroupAddInitiator(igroupName string, initiatorName string) (error)
	IgroupDestroy(igroupName string) (error)
	LunExists(lunPath string) (bool, error)
	IsLunMapped(lunPath string, igroupName string) (bool, error)
	LunGetInfo(lunPath string) (*LunInfo, error)
	LunCopy(imagePath string, lunPath string) (error)
	LunResize(lunPath string, lunSize int) (error)
	LunMap(lunPath string, lunId int, igroupName string) (error)
	LunUnmap(lunPath string, igroupName string) (error)
	LunCreate(lunPath string, lunSize int) (error)
	LunCreateFromFile(volumeName string, filePath string, lunPath string, lunComment string) (error)
	LunDestroy(lunPath string) (error)
	IscsiTargetGetName() (string, error)
	DiscoverIscsiLIFs(lunPath string, initiatorSubnet string) ([]string, error)
	FileExists(volumeName string, filePath string) (bool, error)
	FileDelete(volumName string, filePath string) (error)
	FileDownload(volumeName string, filePath string) ([]byte, error)
	FileUploadAPI(volumeName string, filePath string, reader io.Reader) (error)
	FileUploadNFS(volumeName string, filePath string, reader io.Reader) (error)
	SnapshotGetList(volumeName string) ([]string, error)
	SnapshotCreate(volumeName string, snapshotName string, snapshotComment string) (error)
	SnapshotDelete(volumeName string, snapshotName string) (err error)
	SnapshotRestore(volumeName string, snapshotName string) (error)
}

type LunInfo struct {
	Comment string
	Size int
}

func NewOntapClient(api string, nodeConfig *config.NodeConfig) (ontap OntapClient, err error) {
	switch api {
        case "rest":
                ontap, err = NewOntapRestAPI(nodeConfig)
        case "zapi":
                ontap, err = NewOntapZAPI(nodeConfig)
        default:
                err = fmt.Errorf("NewOntapAPI(): API type \"%s\" is not implemented", api)
        }
	return
}
