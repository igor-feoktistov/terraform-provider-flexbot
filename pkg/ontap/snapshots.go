package ontap

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

// SnapshotExists checks if snapshot exists
func SnapshotExists(nodeConfig *config.NodeConfig, snapshotName string) (exists bool, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("SnapshotExists(): %s", err)
		return
	}
	var snapshots []string
	if snapshots, err = c.SnapshotGetList(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("SnapshotExists(): %s", err)
		return
	}
	exists = false
	for _, snapshot := range snapshots {
		if snapshotName == snapshot {
			exists = true
			break
		}
	}
	return
}

// GetSnapshots gets list of snapshots
func GetSnapshots(nodeConfig *config.NodeConfig) (snapshotList []string, err error) {
	snapshotList = []string{}
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("GetSnapshots(): %s", err)
		return
	}
	if snapshotList, err = c.SnapshotGetList(nodeConfig.Storage.VolumeName); err != nil {
		err = fmt.Errorf("GetSnapshots(): %s", err)
		return
	}
	nodeConfig.Storage.Snapshots = []string{}
	nodeConfig.Storage.Snapshots = append(nodeConfig.Storage.Snapshots, snapshotList...)
	return
}

// CreateSnapshot creates snapshot
func CreateSnapshot(nodeConfig *config.NodeConfig, snapshotName string, snapshotComment string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateSnapshot(): %s", err)
		return
	}
	if err = c.SnapshotCreate(nodeConfig.Storage.VolumeName, snapshotName, snapshotComment); err != nil {
		err = fmt.Errorf("CreateSnapshot(): %s", err)
	}
	return
}

// DeleteSnapshot deletes snapshot
func DeleteSnapshot(nodeConfig *config.NodeConfig, snapshotName string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteSnapshot(): %s", err)
		return
	}
	if err = c.SnapshotDelete(nodeConfig.Storage.VolumeName, snapshotName); err != nil {
		err = fmt.Errorf("DeleteSnapshot(): %s", err)
	}
	return
}

// RestoreSnapshot restores node storage from snapshot
func RestoreSnapshot(nodeConfig *config.NodeConfig, snapshotName string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("RestoreSnapshot(): %s", err)
		return
	}
	if err = c.SnapshotRestore(nodeConfig.Storage.VolumeName, snapshotName); err != nil {
		err = fmt.Errorf("RestoreSnapshot(): %s", err)
	}
	return
}
