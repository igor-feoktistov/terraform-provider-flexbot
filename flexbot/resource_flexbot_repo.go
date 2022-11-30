package flexbot

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
	log "github.com/sirupsen/logrus"
)

func resourceFlexbotRepo() *schema.Resource {
	return &schema.Resource{
		Schema:        schemaFlexbotRepo(),
		CreateContext: resourceCreateRepo,
		ReadContext:   resourceReadRepo,
		UpdateContext: resourceUpdateRepo,
		DeleteContext: resourceDeleteRepo,
		Importer: &schema.ResourceImporter{
			StateContext: resourceImportRepo,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(7200 * time.Second),
			Update: schema.DefaultTimeout(7200 * time.Second),
			Delete: schema.DefaultTimeout(600 * time.Second),
		},
	}
}

func resourceCreateRepo(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	log.Infof("Creating Image Repository")
	if nodeConfig, err = setRepoInput(d, meta); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if err = createRepo(d, nodeConfig, "image_repo"); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if err = createRepo(d, nodeConfig, "template_repo"); err != nil {
		diags = diag.FromErr(err)
		return
	}
	if diags = resourceReadRepo(ctx, d, meta); diags != nil && len(diags) > 0 {
		return
	}
	d.SetId(nodeConfig.Storage.CdotCredentials.Host + ":/repo")
	return
}

func createRepo(d *schema.ResourceData, nodeConfig *config.NodeConfig, repo string) (err error) {
	var repoStorage []string
	if repo == "image_repo" {
		repoStorage, err = ontap.GetRepoImages(nodeConfig)
	}
	if repo == "template_repo" {
		repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
	}
	for _, repoItem := range d.Get(repo).([]interface{}) {
		if err == nil && len(repoItem.(map[string]interface{})["location"].(string)) > 0 {
			if repo == "image_repo" {
				if stringSliceElementExists(repoStorage, repoItem.(map[string]interface{})["name"].(string)) {
					err = ontap.DeleteRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
					time.Sleep(5 * time.Second)
				}
				if err == nil {
					err = ontap.CreateRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string), repoItem.(map[string]interface{})["location"].(string))
				}
			}
			if repo == "template_repo" {
				if stringSliceElementExists(repoStorage, repoItem.(map[string]interface{})["name"].(string)) {
					err = ontap.DeleteRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
					time.Sleep(5 * time.Second)
				}
				if err == nil {
					err = ontap.CreateRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string), repoItem.(map[string]interface{})["location"].(string))
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceCreateRepo(%s): %s", repo, err)
	}
	return
}

func resourceReadRepo(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	log.Infof("Reading Image Repository")
	if nodeConfig, err = setRepoInput(d, meta); err == nil {
		err = setRepoOutput(d, meta, nodeConfig)
	}
	if err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceReadRepo(): %s", err))
	}
	return
}

func resourceUpdateRepo(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	log.Infof("Updating Image Repository")
	if d.HasChange("image_repo") && !d.IsNewResource() {
		err = updateRepo(d, meta, "image_repo")
	}
	if err == nil && d.HasChange("template_repo") && !d.IsNewResource() {
		err = updateRepo(d, meta, "template_repo")
	}
	if err == nil {
		diags = resourceReadRepo(ctx, d, meta)
	} else {
		diags = diag.FromErr(err)
	}
	return
}

func updateRepo(d *schema.ResourceData, meta interface{}, repo string) (err error) {
	var oldRepoState, newRepoState, repoStateInter, repoStorage []string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setRepoInput(d, meta); err != nil {
		return
	}
	oldRepo, newRepo := d.GetChange(repo)
	for _, repoItem := range oldRepo.([]interface{}) {
		oldRepoState = append(oldRepoState, repoItem.(map[string]interface{})["name"].(string))
	}
	for _, repoItem := range newRepo.([]interface{}) {
		newRepoState = append(newRepoState, repoItem.(map[string]interface{})["name"].(string))
	}
	repoStateInter = stringSliceIntersection(oldRepoState, newRepoState)
	if repo == "image_repo" {
		repoStorage, err = ontap.GetRepoImages(nodeConfig)
	}
	if repo == "template_repo" {
		repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
	}
	for _, name := range oldRepoState {
		if err == nil {
			if stringSliceElementExists(repoStorage, name) && !stringSliceElementExists(repoStateInter, name) {
				if repo == "image_repo" {
					err = ontap.DeleteRepoImage(nodeConfig, name)
				}
				if repo == "template_repo" {
					err = ontap.DeleteRepoTemplate(nodeConfig, name)
				}
			}
		}
	}
	if err == nil {
		if repo == "image_repo" {
			repoStorage, err = ontap.GetRepoImages(nodeConfig)
		}
		if repo == "template_repo" {
			repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
		}
	}
	for _, newRepoItem := range newRepo.([]interface{}) {
		if err == nil {
			if len(newRepoItem.(map[string]interface{})["location"].(string)) > 0 {
				locationChanged := true
				for _, oldRepoItem := range oldRepo.([]interface{}) {
					if newRepoItem.(map[string]interface{})["name"].(string) == oldRepoItem.(map[string]interface{})["name"].(string) && newRepoItem.(map[string]interface{})["location"].(string) == oldRepoItem.(map[string]interface{})["location"].(string) {
						locationChanged = false
					}
				}
				if repo == "image_repo" {
					if stringSliceElementExists(repoStorage, newRepoItem.(map[string]interface{})["name"].(string)) && locationChanged {
						err = ontap.DeleteRepoImage(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string))
						time.Sleep(5 * time.Second)
					}
					if err == nil && (!stringSliceElementExists(repoStateInter, newRepoItem.(map[string]interface{})["name"].(string)) || locationChanged) {
						err = ontap.CreateRepoImage(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string), newRepoItem.(map[string]interface{})["location"].(string))
					}
				}
				if repo == "template_repo" {
					if stringSliceElementExists(repoStorage, newRepoItem.(map[string]interface{})["name"].(string)) && locationChanged {
						err = ontap.DeleteRepoTemplate(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string))
						time.Sleep(5 * time.Second)
					}
					if err == nil && (!stringSliceElementExists(repoStateInter, newRepoItem.(map[string]interface{})["name"].(string)) || locationChanged) {
						err = ontap.CreateRepoTemplate(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string), newRepoItem.(map[string]interface{})["location"].(string))
					}
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceUpdateRepo(%s): %s", repo, err)
	}
	return
}

func resourceDeleteRepo(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var nodeConfig *config.NodeConfig
	log.Infof("Deleting Image Repository")
	nodeConfig, err = setRepoInput(d, meta)
	for _, repoItem := range d.Get("image_repo").([]interface{}) {
		if err == nil {
			err = ontap.DeleteRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
		}
	}
	for _, repoItem := range d.Get("template_repo").([]interface{}) {
		if err == nil {
			err = ontap.DeleteRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
		}
	}
	if err != nil {
		diags = diag.FromErr(fmt.Errorf("resourceDeleteRepo(): %s", err))
	} else {
		d.SetId("")
	}
	return
}

func resourceImportRepo(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if diags := resourceReadRepo(ctx, d, meta); diags != nil && len(diags) > 0 {
		return nil, fmt.Errorf("%s: %s", diags[0].Summary, diags[0].Detail)
	}
	return schema.ImportStatePassthroughContext(ctx, d, meta)
}

func setRepoInput(d *schema.ResourceData, meta interface{}) (nodeConfig *config.NodeConfig, err error) {
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	nodeConfig = &config.NodeConfig{}
	p := meta.(*config.FlexbotConfig).FlexbotProvider
	pStorage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := pStorage["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.CdotCredentials.Host = cdotCredentials["host"].(string)
	nodeConfig.Storage.CdotCredentials.User = cdotCredentials["user"].(string)
	nodeConfig.Storage.CdotCredentials.Password = cdotCredentials["password"].(string)
	nodeConfig.Storage.CdotCredentials.ApiMethod = cdotCredentials["api_method"].(string)
	nodeConfig.Storage.CdotCredentials.ZapiVersion = cdotCredentials["zapi_version"].(string)
	if err = config.SetDefaults(nodeConfig, "", "", "", p.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	}
	return
}

func setRepoOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var images, templates []string
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	if images, err = ontap.GetRepoImages(nodeConfig); err == nil {
		d.Set("images", images)
		if templates, err = ontap.GetRepoTemplates(nodeConfig); err == nil {
			d.Set("templates", templates)
		}
	}
	return
}
