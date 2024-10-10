package flexbot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"os"
	"encoding/base64"

	"github.com/denisbrodbeck/machineid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	nodeConfig "github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
)

var (
        // Available rancher_api provides
        RancherApiProviders = []string{"rancher2", "rke", "rke2", "rk-api"}
)

// Provider builds schema
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"pass_phrase": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Default:   "",
			},
			"pass_phrase_env_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Default:   "",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					if len(v) > 0 {
					        _, ok := os.LookupEnv(v)
					        if !ok {
					                errs = append(errs, fmt.Errorf("ENV variable \"%s\" is undefined", v))
					        }
					}
					return
				},
			},
			"synchronized_updates": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"ipam": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"provider": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
								v := val.(string)
								if !(v == "Infoblox" || v == "Internal") {
									errs = append(errs, fmt.Errorf("unsupported %q=%s, allowed values are \"Internal\" and \"Infoblox\"", key, v))
								}
								return
							},
						},
						"credentials": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:      schema.TypeString,
										Required:  true,
										Sensitive: true,
									},
									"wapi_version": {
										Type:     schema.TypeString,
										Required: true,
									},
									"dns_view": {
										Type:     schema.TypeString,
										Required: true,
									},
									"network_view": {
										Type:     schema.TypeString,
										Required: true,
									},
									"ext_attributes": {
										Type:     schema.TypeMap,
										Optional: true,
										Default:  make(map[string]interface{}),
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
						"dns_zone": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"compute": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"credentials": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:      schema.TypeString,
										Required:  true,
										Sensitive: true,
									},
								},
							},
						},
					},
				},
			},
			"storage": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"credentials": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:      schema.TypeString,
										Required:  true,
										Sensitive: true,
									},
									"api_method": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											if !(v == "zapi" || v == "rest") {
												errs = append(errs, fmt.Errorf("unsupported %q=%s, allowed values are \"zapi\" and \"rest\"", key, v))
											}
											return
										},
									},
									"zapi_version": {
										Type:     schema.TypeString,
										Optional: true,
									},
								},
							},
						},
					},
				},
			},
			"rancher_api": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"provider": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "rancher2",
							ValidateFunc: validation.StringInSlice(RancherApiProviders, true),
						},
						"api_url": {
							Type:     schema.TypeString,
							Required: true,
						},
						"token_key": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"server_ca_data": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"client_cert_data": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"client_key_data": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"insecure": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"retries": {
							Optional: true,
							Type:     schema.TypeInt,
							Default:  3,
						},
						"cluster_name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"cluster_id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"wait_for_node_timeout": {
							Optional: true,
							Type:     schema.TypeInt,
							Default:  0,
						},
						"node_grace_timeout": {
							Optional: true,
							Type:     schema.TypeInt,
							Default:  0,
						},
						"drain_input": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"delete_local_data": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  false,
									},
									"force": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  false,
									},
									"grace_period": {
										Type:     schema.TypeInt,
										Optional: true,
										Default:  60,
									},
									"ignore_daemon_sets": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  true,
									},
									"timeout": {
										Type:     schema.TypeInt,
										Optional: true,
										Default:  1800,
									},
								},
							},
						},
					},
				},
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"flexbot_server": resourceFlexbotServer(),
			"flexbot_repo":   resourceFlexbotRepo(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"flexbot_crypt": dataSourceFelxbotCrypt(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var err error
	var diags diag.Diagnostics
	var flexbotConfig *config.FlexbotConfig
	passPhrase := d.Get("pass_phrase").(string)
	if len(passPhrase) == 0 {
	        if passPhrase, err = machineid.ID(); err != nil {
		        diags = append(diags, diag.Diagnostic{
			        Severity: diag.Error,
				Summary:  "providerConfigure(): machineid.ID() failure",
				Detail:   err.Error(),
		        })
			return nil, diags
		}
	}
	if strings.HasPrefix(passPhrase, "base64:") {
	        var passPhraseKey string
	        var passPhraseDecrypted string
	        if len(d.Get("pass_phrase_env_key").(string)) > 0 {
	                passPhraseKey = os.Getenv(d.Get("pass_phrase_env_key").(string))
	        } else {
	                passPhraseKey, err = machineid.ID()
	                if err != nil {
		                diags = append(diags, diag.Diagnostic{
			                Severity: diag.Error,
				        Summary:  "providerConfigure(): machineid.ID() failure",
				        Detail:   err.Error(),
			        })
			        return nil, diags
			}
		}
		if passPhraseDecrypted, err = crypt.DecryptString(passPhrase, passPhraseKey); err != nil {
		        diags = append(diags, diag.Diagnostic{
			        Severity: diag.Error,
			        Summary:  "providerConfigure(): DecryptString() failure",
			        Detail:   err.Error(),
			})
			return nil, diags
		}
		passPhrase = passPhraseDecrypted
	}
	d.Set("pass_phrase", passPhrase)
	if len(d.Get("rancher_api").([]interface{})) > 0 {
		rancherAPI := d.Get("rancher_api").([]interface{})[0].(map[string]interface{})
		ignoreDaemonSets := rancherAPI["drain_input"].([]interface{})[0].(map[string]interface{})["ignore_daemon_sets"].(bool)
		nodeDrainInput := &config.NodeDrainInput{
			Force:            rancherAPI["drain_input"].([]interface{})[0].(map[string]interface{})["force"].(bool),
			IgnoreDaemonSets: &ignoreDaemonSets,
			DeleteLocalData:  rancherAPI["drain_input"].([]interface{})[0].(map[string]interface{})["delete_local_data"].(bool),
			GracePeriod:      int64(rancherAPI["drain_input"].([]interface{})[0].(map[string]interface{})["grace_period"].(int)),
			Timeout:          int64(rancherAPI["drain_input"].([]interface{})[0].(map[string]interface{})["timeout"].(int)),
		}
		var tokenKey, decrypted string
		var serverCAData, clientCertData, clientKeyData []byte
                if len(rancherAPI["token_key"].(string)) > 0 {
		        if tokenKey, err = crypt.DecryptString(rancherAPI["token_key"].(string), passPhrase); err != nil {
		                diags = append(diags, diag.Diagnostic{
			                Severity: diag.Error,
			                Summary:  "providerConfigure(): DecryptString(token_key) failure",
			                Detail:   err.Error(),
		                })
		                return nil, diags
		        }
		}
                if len(rancherAPI["server_ca_data"].(string)) > 0 {
		        if decrypted, err = crypt.DecryptString(rancherAPI["server_ca_data"].(string), passPhrase); err == nil {
		                serverCAData, err = base64.StdEncoding.DecodeString(decrypted)
		        }
		        if err != nil {
		                diags = append(diags, diag.Diagnostic{
			                Severity: diag.Error,
			                Summary:  "providerConfigure(): DecryptString(server_ca_data) failure",
			                Detail:   err.Error(),
		                })
		                return nil, diags
		        }
		}
                if len(rancherAPI["client_cert_data"].(string)) > 0 {
		        if decrypted, err = crypt.DecryptString(rancherAPI["client_cert_data"].(string), passPhrase); err == nil {
		                clientCertData, err = base64.StdEncoding.DecodeString(decrypted)
		        }
		        if err != nil {
		                diags = append(diags, diag.Diagnostic{
			                Severity: diag.Error,
			                Summary:  "providerConfigure(): DecryptString(client_cert_data) failure",
			                Detail:   err.Error(),
		                })
		                return nil, diags
		        }
		}
                if len(rancherAPI["client_key_data"].(string)) > 0 {
		        if decrypted, err = crypt.DecryptString(rancherAPI["client_key_data"].(string), passPhrase); err == nil {
		                clientKeyData, err = base64.StdEncoding.DecodeString(decrypted)
		        }
		        if err != nil {
		                diags = append(diags, diag.Diagnostic{
			                Severity: diag.Error,
			                Summary:  "providerConfigure(): DecryptString(client_key_data) failure",
			                Detail:   err.Error(),
		                })
		                return nil, diags
		        }
		}
		rancherConfig := &config.RancherConfig{
		        Provider:           rancherAPI["provider"].(string),
			URL:                rancherAPI["api_url"].(string),
			TokenKey:           tokenKey,
			ServerCAData:       serverCAData,
			ClientCertData:     clientCertData,
			ClientKeyData:      clientKeyData,
			Insecure:           rancherAPI["insecure"].(bool),
			NodeDrainInput:     nodeDrainInput,
			Retries:            rancherAPI["retries"].(int),
		}
		flexbotConfig = &config.FlexbotConfig{
			Sync:               &sync.Mutex{},
			FlexbotProvider:    d,
			RancherApiEnabled:  rancherAPI["enabled"].(bool),
			RancherConfig:      rancherConfig,
			WaitForNodeTimeout: rancherAPI["wait_for_node_timeout"].(int),
			NodeGraceTimeout:   rancherAPI["node_grace_timeout"].(int),
			NodeConfig:         make(map[string]*nodeConfig.NodeConfig),
		}
	} else {
		flexbotConfig = &config.FlexbotConfig{
			Sync:              &sync.Mutex{},
			FlexbotProvider:   d,
			RancherApiEnabled: false,
			NodeConfig:        make(map[string]*nodeConfig.NodeConfig),
		}
	}
	return flexbotConfig, diags
}
