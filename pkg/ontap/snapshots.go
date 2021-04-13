package ontap

import (
	"fmt"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

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
	for _, snapshot := range snapshotList {
		nodeConfig.Storage.Snapshots = append(nodeConfig.Storage.Snapshots, snapshot)
	}
	return
}

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
