package ontap

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
	"github.com/kdomanski/iso9660"
)

// CreateSeedStorage creates cloud-init ISO LUN for the node
func CreateSeedStorage(nodeConfig *config.NodeConfig) (err error) {
	var fileReader io.Reader
	var file *os.File
	var b []byte
	if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(nodeConfig.Storage.SeedLun.SeedTemplate.Location); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateSeedStorage(): failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	} else {
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			if os.IsNotExist(err) {
				// Last resort to download template from storage repository
				b, err = DownloadRepoTemplate(nodeConfig, filepath.Base(nodeConfig.Storage.SeedLun.SeedTemplate.Location))
			}
		} else {
			fileReader = file
			defer file.Close()
		}
		if err != nil {
			err = fmt.Errorf("CreateSeedStorage(): failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	}
	if len(b) == 0 {
		if b, err = ioutil.ReadAll(fileReader); err != nil {
			err = fmt.Errorf("CreateSeedStorage(): failure to read cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	}
	var isoWriter *iso9660.ImageWriter
	if isoWriter, err = iso9660.NewWriter(); err != nil {
		err = fmt.Errorf("CreateSeedStorage(): failed to create ISO writer: %v", err)
		return
	}
	defer isoWriter.Cleanup()
	for _, cloudInitData := range []string{"meta-data", "network-config", "user-data"} {
		var cloudInitBuf bytes.Buffer
		var t *template.Template
		if t, err = template.New(cloudInitData).Parse(string(b)); err != nil {
			err = fmt.Errorf("CreateSeedStorage(): failure to parse cloud-init template: %s", err)
			return
		}
		if err = t.Execute(&cloudInitBuf, nodeConfig); err != nil {
			err = fmt.Errorf("CreateSeedStorage(): template failure for %s: %s", cloudInitData, err)
			return
		}
		cloudInitReader := strings.NewReader(cloudInitBuf.String())
		if err = isoWriter.AddFile(cloudInitReader, cloudInitData); err != nil {
			err = fmt.Errorf("CreateSeedStorage(): failed to add %s file to ISO: %v", cloudInitData, err)
			return
		}
	}
	var isoBuffer bytes.Buffer
	if err = isoWriter.WriteTo(&isoBuffer, "cidata"); err != nil {
		err = fmt.Errorf("CreateSeedStorage(): failed to write ISO image: %v", err)
		return
	}
	isoReader := bytes.NewReader(isoBuffer.Bytes())
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateSeedStorage(): %s", err)
		return
	}
	seedLunPath := "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name
	var lunExists bool
	if lunExists, err = c.LunExists(seedLunPath); err != nil {
	        err = fmt.Errorf("CreateSeedStorage(): %s", err)
		return
	}
	if lunExists {
	        _ = c.LunUnmap(seedLunPath, nodeConfig.Storage.IgroupName)
		if err = c.LunDestroy(seedLunPath); err != nil {
		        err = fmt.Errorf("CreateSeedStorage(): %s", err)
			return
		}
	}
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.VolumeName, "/seed"); err == nil && fileExists {
		if err = c.FileDelete(nodeConfig.Storage.VolumeName, "/seed"); err != nil {
			err = fmt.Errorf("CreateSeedStorage(): %s", err)
			return
		}
	}
	if err = c.LunCreateAndUpload(nodeConfig.Storage.VolumeName, "/seed", int64(isoBuffer.Len()), isoReader, seedLunPath, nodeConfig.Storage.SeedLun.SeedTemplate.Location); err != nil {
		err = fmt.Errorf("CreateSeedStorage(): %s", err)
		return
        }
	if err = c.LunMap(seedLunPath, nodeConfig.Storage.SeedLun.Id, nodeConfig.Storage.IgroupName); err != nil {
		err = fmt.Errorf("CreateSeedStorage(): %s", err)
	}
	return
}

// CreateSeedStoragePreflight is sanity check before actual storage is created
func CreateSeedStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(nodeConfig.Storage.SeedLun.SeedTemplate.Location); err == nil {
			httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateSeedStoragePreflight(): failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
		}
	} else {
		var file *os.File
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			templateExists := false
			if os.IsNotExist(err) {
				templateExists, err = RepoTemplateExists(nodeConfig, nodeConfig.Storage.SeedLun.SeedTemplate.Location)
			}
			if !templateExists {
				err = fmt.Errorf("CreateSeedStoragePreflight(): failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			}
		} else {
			file.Close()
		}
	}
	return
}
