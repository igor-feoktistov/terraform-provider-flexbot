package ontap

import (
	"fmt"
	"os"
	"os/exec"
	"io"
        "io/ioutil"
        "net/http"
	"strings"
	"bytes"
	"path/filepath"
	"text/template"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

// CreateEsxStorage creates ESX node storage in cDOT
func CreateEsxStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	var tmpDir, overlayTmpDir string
	errorFormat := "CreateEsxStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
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
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	var lunExists bool
	if lunExists, err = c.LunExists(bootLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunExists {
		if err = c.LunCreate(bootLunPath, nodeConfig.Storage.BootLun.Size, "vmware"); err != nil {
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
	if tmpDir, err = os.MkdirTemp("", nodeConfig.Storage.BootLun.Name + "-*"); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to create temp dir: %s", err)
		return
	}
	defer os.RemoveAll(tmpDir)
	if overlayTmpDir, err = os.MkdirTemp("", nodeConfig.Storage.BootLun.Name + "-iso-overlay-*"); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to create temp dir: %s", err)
		return
	}
	defer os.RemoveAll(overlayTmpDir)
	bootImagePath := tmpDir + "/" + nodeConfig.Storage.BootLun.OsImage.Name
	var fileReader io.Reader
	var file *os.File
	if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(nodeConfig.Storage.SeedLun.SeedTemplate.Location); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateEsxStorage(): failure to get kickstart template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	} else {
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			err = fmt.Errorf("CreateEsxStorage(): failure to open kickstart template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
		fileReader = file
		defer file.Close()
	}
	var b []byte
	if b, err = ioutil.ReadAll(fileReader); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to read kickstart template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
		return
	}
	var ksBuf bytes.Buffer
	var t *template.Template
	if t, err = template.New("ks").Parse(string(b)); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to parse kickstart template: %s", err)
		return
	}
	if err = t.Execute(&ksBuf, nodeConfig); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to execute kickstart template: %s", err)
		return
	}
	ksBytes := ksBuf.Bytes()
	if err = os.WriteFile(filepath.Join(overlayTmpDir, "ks.cfg"), ksBytes, 0644); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to write to ks.cfg: %s", err)
		return
	}
	cmd := exec.Command("tar", "czf", filepath.Join(overlayTmpDir, "OEM.TGZ"), "ks.cfg")
	cmd.Dir = overlayTmpDir
	var cmdOutput []byte
	if cmdOutput, err = cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): TAR failure: %s:%s", err, cmdOutput)
		return
	}
	os.Remove(filepath.Join(overlayTmpDir, "ks.cfg"))
	var bootCfg []byte
	if bootCfg, err = extractISOFile(nodeConfig.Storage.BootLun.OsImage.Location, "/BOOT.CFG"); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to extract boot.cfg: %s", err)
		return
	}
	content := strings.ReplaceAll(string(bootCfg), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "kernelopt=") {
			lines[i] = "kernelopt=" + nodeConfig.Compute.KernelOpt
		}
		if strings.HasPrefix(line, "modules=") {
			lines[i] = line + " --- /oem.tgz"
		}
	}
	bootCfgModified := []byte(strings.Join(lines, "\n"))
	if err = os.WriteFile(filepath.Join(overlayTmpDir, "BOOT.CFG"), bootCfgModified, 0644); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to write to boot.cfg: %s", err)
		return
	}
	efiDir := filepath.Join(overlayTmpDir, "EFI", "BOOT")
	if err = os.MkdirAll(efiDir, 0755); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): mkdir EFI: %s", err)
		return
	}
	if err = os.WriteFile(filepath.Join(efiDir, "BOOT.CFG"), bootCfgModified, 0644); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failute to write to EFI boot.cfg: %s", err)
		return
	}
	if err = buildISO(nodeConfig.Storage.BootLun.OsImage.Location, bootImagePath, overlayTmpDir, nodeConfig.Compute.Firmware); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): buildISO(): %s", err)
		return
	}
	if file, err = os.Open(bootImagePath); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to open file %s: %s", bootImagePath, err)
		return
	}
	var fileInfo os.FileInfo
	if fileInfo, err = file.Stat(); err != nil {
		err = fmt.Errorf("CreateEsxStorage(): failure to get stat() for file %s: %s", bootImagePath, err)
		return
	}
	imageSize := int64(fileInfo.Size())
	if err = c.LunUpload(bootLunPath, file, imageSize); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
        }
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !igroupExists {
		if c.IgroupCreate(nodeConfig.Storage.IgroupName, "vmware"); err != nil {
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
	var lunMapped bool
	if lunMapped, err = c.IsLunMapped(bootLunPath, nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if !lunMapped {
		if err = c.LunMap(bootLunPath, 0, nodeConfig.Storage.IgroupName); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
		}
	}
	return
}

// CreateEsxStoragePreflight is sanity check before actual storage provisioning
func CreateEsxStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	errorFormat := "CreateEsxStoragePreflight(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if _, err = c.GetAggregateMax(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
	}
	if _, err = exec.LookPath("xorriso"); err != nil {
		err = fmt.Errorf("CreateEsxStoragePreflight(): xorriso not found: %s ", err)
	}
	if _, err = exec.LookPath("isohybrid"); err != nil {
		err = fmt.Errorf("CreateEsxStoragePreflight(): isohybrid not found: %s ", err)
	}
	var imageFile, templateFile *os.File
	if !(strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://")) {
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			templateFile, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			templateFile, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			err = fmt.Errorf("CreateEsxStoragePreflight(): failure to open kickstart template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
		defer templateFile.Close()
	}
	if !(strings.HasPrefix(nodeConfig.Storage.BootLun.OsImage.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.BootLun.OsImage.Location, "https://")) {
		if strings.HasPrefix(nodeConfig.Storage.BootLun.OsImage.Location, "file://") {
			imageFile, err = os.Open(nodeConfig.Storage.BootLun.OsImage.Location[7:])
		} else {
			imageFile, err = os.Open(nodeConfig.Storage.BootLun.OsImage.Location)
		}
		if err != nil {
			err = fmt.Errorf("CreateEsxStoragePreflight(): failure to open image %s: %s", nodeConfig.Storage.BootLun.OsImage.Location, err)
			return
		}
		defer imageFile.Close()
	}
	return
}

// DeleteEsxStorage deletes ESX node storage
func DeleteEsxStorage(nodeConfig *config.NodeConfig) (err error) {
	var c client.OntapClient
	errorFormat := "DeleteEsxStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	bootLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.BootLun.Name
	var igroupExists bool
	if igroupExists, err = c.IgroupExists(nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	var lunExists bool
	if lunExists, err = c.LunExists(bootLunPath); err != nil {
		err = fmt.Errorf(errorFormat, err)
		return
	}
	if lunExists {
		if igroupExists {
			if err = c.LunUnmap(bootLunPath, nodeConfig.Storage.IgroupName); err != nil {
				err = fmt.Errorf(errorFormat, err)
				return
			}
		}
		if err = c.LunDestroy(bootLunPath); err != nil {
			err = fmt.Errorf(errorFormat, err)
			return
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
		if err = c.VolumeDestroy(nodeConfig.Storage.VolumeName); err != nil {
			err = fmt.Errorf(errorFormat, err)
		}
	}
	return
}

// DiscoverEsxStorage discovers ESX storage in cDOT
func DiscoverEsxStorage(nodeConfig *config.NodeConfig) (storageExists bool, err error) {
	var c client.OntapClient
	errorFormat := "DiscoverEsxStorage(): %s"
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DiscoverEsxStorage(): %s", err)
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

// extractISOFile extracts a single file from an ISO using xorriso.
func extractISOFile(isoPath, isoFilePath string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "iso-extract-*")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	cmd := exec.Command("xorriso",
		"-indev", isoPath,
		"-osirrox", "on",
		"-extract", isoFilePath, tmp.Name(),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("xorriso: %v: %s", err, out)
	}
	return os.ReadFile(tmp.Name())
}

// buildISO creates a new ISO by overlaying modified files onto the source ISO.
func buildISO(inputISO, outputISO, overlayDir string, firmware string) (err error) {
	var args []string
	var cmd *exec.Cmd
	args = []string{
		"-indev", inputISO,
		"-outdev", outputISO,
		"-overwrite", "on",
		"-map", overlayDir, "/",
		"-boot_image", "any", "replay",
	}
	cmd = exec.Command("xorriso", args...)
	if err = cmd.Run(); err == nil {
		if firmware == "bios" {
			args = []string{
				outputISO,
			}
			cmd = exec.Command("isohybrid", args...)
			if err = cmd.Run(); err != nil {
				err = fmt.Errorf("isohybrid: %s", err)
			}
		}
	} else {
		err = fmt.Errorf("xorriso: %s", err)
	}
	return
}

// extractISOFileToPath extracts a file from ISO to a local path using xorriso.
func extractISOFileToPath(isoPath, isoFilePath, localPath string) error {
	cmd := exec.Command("xorriso",
		"-indev", isoPath,
		"-osirrox", "on",
		"-extract", isoFilePath, localPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}
