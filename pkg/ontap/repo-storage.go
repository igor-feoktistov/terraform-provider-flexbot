package ontap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap/client"
)

const (
	imageRepoVolSize    = 64
	templateRepoVolSize = 1
)

// CreateRepoImage creates cDOT storage and uploads image
func CreateRepoImage(nodeConfig *config.NodeConfig, imageName string, imagePath string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateRepoImage(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("CreateRepoImage(): %s", err)
		return
	}
	if !volExists {
		var aggregateName string
		if aggregateName, err = c.GetAggregateMax(nodeConfig); err != nil {
			err = fmt.Errorf("CreateRepoImage(): %s", err)
			return
		}
		if err = c.ExportPolicyCreate(nodeConfig.Storage.ImageRepoName); err != nil {
			err = fmt.Errorf("CreateRepoImage(): %s", err)
			return
		}
		if err = c.VolumeCreateNAS(nodeConfig.Storage.ImageRepoName, aggregateName, nodeConfig.Storage.ImageRepoName, imageRepoVolSize); err != nil {
			err = fmt.Errorf("CreateRepoImage(): %s", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
	var lunExists bool
	if lunExists, err = c.LunExists("/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName); err != nil {
		err = fmt.Errorf("CreateRepoImage(): %s", err)
		return
	}
	if lunExists {
		if err = c.LunDestroy("/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName); err != nil {
			err = fmt.Errorf("CreateRepoImage(): %s", err)
			return
		}
	}
	var fileReader io.Reader
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.ImageRepoName, "/_"+imageName); err != nil {
		err = fmt.Errorf("CreateRepoImage(): %s", err)
		return
	}
	if fileExists {
		if err = c.FileDelete(nodeConfig.Storage.ImageRepoName, "/_"+imageName); err != nil {
			err = fmt.Errorf("CreateRepoImage(): %s", err)
			return
		}
	}
	var imageSize int64
	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(imagePath); err == nil {
			fileReader = httpResponse.Body
			imageSize = 0
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateRepoImage(): failure to open file %s: %s", imagePath, err)
			return
		}
	} else {
		var file *os.File
		var fileInfo os.FileInfo
		if strings.HasPrefix(imagePath, "file://") {
			file, err = os.Open(imagePath[7:])
		} else {
			file, err = os.Open(imagePath)
		}
		if err != nil {
			err = fmt.Errorf("CreateRepoImage(): failure to open file %s: %s", imagePath, err)
			return
		}
		fileReader = file
		if fileInfo, err = file.Stat(); err != nil {
			err = fmt.Errorf("CreateRepoImage(): failure in Stat() for file %s: %s", imagePath, err)
			return
		}
		imageSize = int64(fileInfo.Size())
		defer file.Close()
	}
	if err = c.LunCreateAndUpload(nodeConfig.Storage.ImageRepoName, "/_"+imageName, imageSize, fileReader, "/vol/"+nodeConfig.Storage.ImageRepoName+"/"+imageName, imageName); err != nil {
		err = fmt.Errorf("CreateRepoImage(): %s", err)
		return
        }
	return
}

// DeleteRepoImage deletes all related to the image cDOT storage elements
func DeleteRepoImage(nodeConfig *config.NodeConfig, imageName string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteRepoImage(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("DeleteRepoImage(): %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DeleteRepoImage(): repo volume \"%s\" does not exist", nodeConfig.Storage.ImageRepoName)
		return
	}
	var lunExists bool
	if lunExists, err = c.LunExists("/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName); err != nil {
		err = fmt.Errorf("DeleteRepoImage(): %s", err)
		return
	}
	if lunExists {
		if err = c.LunDestroy("/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName); err != nil {
			err = fmt.Errorf("DeleteBootImage(): %s", err)
			return
		}
	}
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.ImageRepoName, "/_"+imageName); err != nil {
		err = fmt.Errorf("DeleteRepoImage(): %s", err)
		return
	}
	if fileExists {
		if err = c.FileDelete(nodeConfig.Storage.ImageRepoName, "/_"+imageName); err != nil {
			err = fmt.Errorf("DeleteRepoImage(): %s", err)
		}
	}
	return
}

// GetRepoImages gets list of images
func GetRepoImages(nodeConfig *config.NodeConfig) (imagesList []string, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("GetRepoImages(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("GetRepoImages(): %s", err)
		return
	}
	if !volExists {
		return
	}
	if imagesList, err = c.LunGetList(nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("GetRepoImages(): %s", err)
	}
	return
}

// CreateRepoTemplate creates cDOT storage and uploads cloud-init template
func CreateRepoTemplate(nodeConfig *config.NodeConfig, templateName string, templatePath string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateRepoTemplate(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("CreateRepoTemplate(): %s", err)
		return
	}
	if !volExists {
		var aggregateName string
		if aggregateName, err = c.GetAggregateMax(nodeConfig); err != nil {
			err = fmt.Errorf("CreateRepoTemplate(): %s", err)
			return
		}
		if err = c.ExportPolicyCreate(nodeConfig.Storage.TemplateRepoName); err != nil {
			err = fmt.Errorf("CreateRepoTemplate(): %s", err)
			return
		}
		if err = c.VolumeCreateNAS(nodeConfig.Storage.TemplateRepoName, aggregateName, nodeConfig.Storage.TemplateRepoName, templateRepoVolSize); err != nil {
			err = fmt.Errorf("CreateRepoTemplate(): %s", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
	var fileReader io.Reader
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
		err = fmt.Errorf("CreateRepoTemplate(): %s", err)
		return
	}
	if fileExists {
		if err = c.FileDelete(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
			err = fmt.Errorf("CreateRepoTemplate(): %s", err)
			return
		}
	}
	if strings.HasPrefix(templatePath, "http://") || strings.HasPrefix(templatePath, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(templatePath); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateRepoTemplate(): failure to open file %s: %s", templatePath, err)
			return
		}
	} else {
		var file *os.File
		if strings.HasPrefix(templatePath, "file://") {
			file, err = os.Open(templatePath[7:])
		} else {
			file, err = os.Open(templatePath)
		}
		if err != nil {
			err = fmt.Errorf("CreateRepoTemplate(): failure to open file %s: %s", templatePath, err)
			return
		}
		fileReader = file
		defer file.Close()
	}
	if err = c.FileUploadAPI(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName, fileReader); err != nil {
		err = fmt.Errorf("CreateRepoTemplate(): %s", err)
	}
	return
}

// GetRepoTemplates get list of cloud-init templates
func GetRepoTemplates(nodeConfig *config.NodeConfig) (templatesList []string, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("GetRepoTemplates(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("GetRepoTemplates(): %s", err)
		return
	}
	if !volExists {
		return
	}
	if templatesList, err = c.FileGetList(nodeConfig.Storage.TemplateRepoName, "/cloud-init"); err != nil {
		err = fmt.Errorf("GetRepoTemplates(): %s", err)
	}
	return
}

// DeleteRepoTemplate deletes all related to the cloud-init template storage elements
func DeleteRepoTemplate(nodeConfig *config.NodeConfig, templateName string) (err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate(): %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DeleteRepoTemplate(): repo volume \"%s\" does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	var fileExists bool
	if fileExists, err = c.FileExists(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate(): %s", err)
		return
	}
	if fileExists {
		if err = c.FileDelete(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
			err = fmt.Errorf("DeleteRepoTemplate(): %s", err)
		}
	}
	return
}

// RepoTemplateExists checks if cloud-init template exists
func RepoTemplateExists(nodeConfig *config.NodeConfig, templateName string) (templateExists bool, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("RepoTemplateExists(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("RepoTemplateExists(): %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("RepoTemplateExists(): repo volume \"%s\" does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	if templateExists, err = c.FileExists(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
		err = fmt.Errorf("RepoTemplateExists(): %s", err)
		return
	}
	return
}

// DownloadRepoTemplate downloads cloud-init template from cDOT storage
func DownloadRepoTemplate(nodeConfig *config.NodeConfig, templateName string) (templateContent []byte, err error) {
	var c client.OntapClient
	if c, err = client.NewOntapClient(nodeConfig); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate(): %s", err)
		return
	}
	var volExists bool
	if volExists, err = c.VolumeExists(nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate(): %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DownloadRepoTemplate(): repo volume \"%s\" does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	if templateContent, err = c.FileDownload(nodeConfig.Storage.TemplateRepoName, "/cloud-init/"+templateName); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate(): %s", err)
	}
	return
}
