package client

import (
	"fmt"
	"io"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
)

// OntapClient is generic cDOT client interface
type OntapClient interface {
	GetAggregateMax(nodeConfig *config.NodeConfig) (string, error)
	VolumeExists(volumeName string) (bool, error)
	VolumeCreateSAN(volumeName string, aggregateName string, volumeSize int) error
	VolumeCreateNAS(volumeName string, aggregateName string, exportPolicyName string, volumeSize int) error
	VolumeDestroy(volumeName string) error
	VolumeResize(volumeName string, volumeSize int) error
	ExportPolicyCreate(exportPolicyName string) error
	IgroupExists(volumeName string) (bool, error)
	IgroupCreate(igroupName string) error
	IgroupAddInitiator(igroupName string, initiatorName string) error
	IgroupDestroy(igroupName string) error
	LunExists(lunPath string) (bool, error)
	IsLunMapped(lunPath string, igroupName string) (bool, error)
	LunGetInfo(lunPath string) (*LunInfo, error)
	LunGetList(volumeName string) ([]string, error)
	LunCopy(imagePath string, lunPath string) error
	LunResize(lunPath string, lunSize int) error
	LunMap(lunPath string, lunID int, igroupName string) error
	LunUnmap(lunPath string, igroupName string) error
	LunCreate(lunPath string, lunSize int) error
	LunCreateFromFile(volumeName string, filePath string, lunPath string, lunComment string) error
	LunCreateAndUpload(volumeName string, filePath string, fileSize int64, fileReader io.Reader, lunPath string, lunComment string) error
	LunDestroy(lunPath string) error
	IscsiTargetGetName() (string, error)
	DiscoverIscsiLIFs(lunPath string, initiatorSubnet string) ([]string, error)
	FileExists(volumeName string, filePath string) (bool, error)
	FileGetList(volumeName string, dirPath string) ([]string, error)
	FileDelete(volumName string, filePath string) error
	FileDownload(volumeName string, filePath string) ([]byte, error)
	FileUploadAPI(volumeName string, filePath string, reader io.Reader) error
	FileUploadNFS(volumeName string, filePath string, reader io.Reader) error
	SnapshotGetList(volumeName string) ([]string, error)
	SnapshotCreate(volumeName string, snapshotName string, snapshotComment string) error
	SnapshotDelete(volumeName string, snapshotName string) (err error)
	SnapshotRestore(volumeName string, snapshotName string) error
}

// LunInfo is generic LUN info
type LunInfo struct {
	Comment string
	Size    int
}

// NewOntapClient creates cDOT client
func NewOntapClient(nodeConfig *config.NodeConfig) (ontap OntapClient, err error) {
	switch nodeConfig.Storage.CdotCredentials.ApiMethod {
	case "rest":
		ontap, err = NewOntapRestAPI(nodeConfig)
	case "zapi":
		ontap, err = NewOntapZAPI(nodeConfig)
	default:
		err = fmt.Errorf("NewOntapAPI(): API method \"%s\" is not implemented", nodeConfig.Storage.CdotCredentials.ApiMethod)
	}
	return
}
