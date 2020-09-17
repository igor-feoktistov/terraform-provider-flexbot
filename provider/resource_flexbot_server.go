package provider

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"bytes"
	"sync"
	"time"
	"golang.org/x/crypto/ssh"

	"flexbot/pkg/config"
	"flexbot/pkg/ipam"
	"flexbot/pkg/ontap"
	"flexbot/pkg/ucsm"
	"github.com/denisbrodbeck/machineid"
	"github.com/hashicorp/terraform/helper/schema"
)

var setInputMutex = sync.Mutex{}
var setOutputMutex = sync.Mutex{}

func resourceFlexbotServer() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"compute": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hostname": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"sp_org": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"sp_template": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"sp_dn": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"safe_removal": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
						"wait_for_ssh_timeout": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  0,
						},
						"ssh_user": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"ssh_private_key": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"blade_spec": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"dn": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"model": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"num_of_cpus": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^[0-9-]+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be either number or range", key, v))
											}
											return
										},
									},
									"num_of_cores": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^[0-9-]+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be either number or range", key, v))
											}
											return
										},
									},
									"total_memory": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^[0-9-]+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be either number or range", key, v))
											}
											return
										},
									},
								},
							},
						},
					},
				},
			},
			"storage": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"svm_name": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"image_repo_name": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"volume_name": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"igroup_name": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"boot_lun": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"id": {
										Type:     schema.TypeInt,
										Optional: true,
										Computed: true,
									},
									"size": {
										Type:     schema.TypeInt,
										Required: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(int)
											if v < 0 || v > 1024 {
												errs = append(errs, fmt.Errorf("%q must be between 0 and 1024 inclusive, got: %d", key, v))
											}
											return
										},
									},
									"os_image": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"seed_lun": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"id": {
										Type:     schema.TypeInt,
										Optional: true,
										Computed: true,
									},
									"seed_template": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"data_lun": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"id": {
										Type:     schema.TypeInt,
										Optional: true,
										Computed: true,
									},
									"size": {
										Type:     schema.TypeInt,
										Required: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(int)
											if v < 0 || v > 4096 {
												errs = append(errs, fmt.Errorf("%q must be between 0 and 4096 inclusive, got: %d", key, v))
											}
											return
										},
									},
								},
							},
						},
					},
				},
			},
			"network": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"node": {
							Type:     schema.TypeList,
							Required: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"macaddr": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"ip": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
											return
										},
									},
									"fqdn": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"subnet": {
										Type:     schema.TypeString,
										Required: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\/\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("subnet %q=%s must be in CIDR format", key, v))
											}
											return
										},
									},
									"gateway": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											if len(v) > 0 {
												matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
												if !matched {
													errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
												}
											}
											return
										},
									},
									"dns_server1": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											if len(v) > 0 {
												matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
												if !matched {
													errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
												}
											}
											return
										},
									},
									"dns_server2": {
										Type:     schema.TypeString,
										Optional: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											if len(v) > 0 {
												matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
												if !matched {
													errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
												}
											}
											return
										},
									},
									"dns_domain": {
										Type:     schema.TypeString,
										Optional: true,
									},
								},
							},
						},
						"iscsi_initiator": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 2,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"ip": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
											return
										},
									},
									"fqdn": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"subnet": {
										Type:     schema.TypeString,
										Required: true,
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\/\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("subnet %q=%s must be in CIDR format", key, v))
											}
											return
										},
									},
									"gateway": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "0.0.0.0",
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
											return
										},
									},
									"dns_server1": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "0.0.0.0",
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
											return
										},
									},
									"dns_server2": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "0.0.0.0",
										ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
											v := val.(string)
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
											return
										},
									},
									"initiator_name": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
									"iscsi_target": {
										Type:     schema.TypeList,
										Optional: true,
										Computed: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"node_name": {
													Type:     schema.TypeString,
													Optional: true,
													Computed: true,
												},
												"interfaces": {
													Type:     schema.TypeList,
													Optional: true,
													Computed: true,
													Elem:     &schema.Schema{Type: schema.TypeString},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"cloud_args": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"snapshot": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"fsfreeze": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},
		},
		Create: resourceCreateServer,
		Read:   resourceReadServer,
		Update: resourceUpdateServer,
		Delete: resourceDeleteServer,
		Importer: &schema.ResourceImporter{
			State: resourceImportServer,
		},
	}
}

func resourceCreateServer(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig

	p := meta.(*schema.ResourceData)
	if nodeConfig, err = setFlexbotInput(d, p); err != nil {
		return
	}
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	for _, snapshot := range d.Get("snapshot").([]interface{}) {
		name := snapshot.(map[string]interface{})["name"].(string)
		if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
			if compute["wait_for_ssh_timeout"].(int) == 0 {
				err = fmt.Errorf("resourceCreateServer(): expected \"compute.wait_for_ssh_timeout\" parameter to ensure fsfreeze for snapshot \"%s\"", name)
				return
			}
			if len(sshUser) == 0 || len(sshPrivateKey) == 0 {
				err = fmt.Errorf("resourceCreateServer(): expected \"compute.ssh_user\" and \"compute.ssh_private_key\" parameters to ensure fsfreeze for snapshot \"%s\"", name)
				return
			}
		}
	}
	log.Printf("[INFO] Creating Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	if serverExists {
		err = fmt.Errorf("resourceCreateServer(): serverServer \"%s\" already exists", nodeConfig.Compute.HostName)
		return
	}
	var provider ipam.IpamProvider
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("resourceCreateServer(): IPAM provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
		return
	}
	var preflightErr error
	preflightErrMsg := []string{}
	preflightErr = provider.AllocatePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ontap.CreateBootStoragePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ucsm.CreateServerPreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ontap.CreateSeedStoragePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	if len(preflightErrMsg) > 0 {
		err = fmt.Errorf("resourceCreateServer(): %s", strings.Join(preflightErrMsg, "\n"))
		return
	}
	if err = provider.Allocate(nodeConfig); err != nil {
		err = fmt.Errorf("resourceCreateServer(): %s", err)
		return
	}
	if err = ontap.CreateBootStorage(nodeConfig); err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		err = ontap.CreateSeedStorage(nodeConfig)
	}
	if err == nil {
		err = ucsm.StartServer(nodeConfig)
	}
	d.SetId(nodeConfig.Compute.HostName)
	if err == nil {
		d.SetConnInfo(map[string]string{
			"type": "ssh",
			"host": nodeConfig.Network.Node[0].Ip,
		})
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && err == nil {
		giveupTime := time.Now().Add(time.Second * time.Duration(compute["wait_for_ssh_timeout"].(int)))
		restartTime := time.Now().Add(time.Second * 600)
		for time.Now().Before(giveupTime) {
			if checkSshListen(nodeConfig.Network.Node[0].Ip) {
				if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
					stabilazeTime := time.Now().Add(time.Second * 60)
					for time.Now().Before(stabilazeTime) {
						if err = checkSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey); err == nil {
							break
						}
						time.Sleep(1 * time.Second)
					}
				}
				break
			}
			time.Sleep(1 * time.Second)
			if time.Now().After(restartTime) {
				ucsm.StopServer(nodeConfig)
				ucsm.StartServer(nodeConfig)
				restartTime = time.Now().Add(time.Second * 600)
			}
		}
	}
	if err == nil {
		for _, snapshot := range d.Get("snapshot").([]interface{}) {
			name := snapshot.(map[string]interface{})["name"].(string)
			if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
				err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
			} else {
				err = ontap.CreateSnapshot(nodeConfig, name)
			}
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceCreateServer(): %s", err)
	}
	setFlexbotOutput(d, nodeConfig)
	return
}

func resourceReadServer(d *schema.ResourceData, meta interface{}) (err error) {
	p := meta.(*schema.ResourceData)
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, p); err != nil {
		return
	}
	log.Printf("[INFO] Refreshing Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	if serverExists {
		var provider ipam.IpamProvider
		switch nodeConfig.Ipam.Provider {
		case "Infoblox":
			provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
		case "Internal":
			provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
		default:
			err = fmt.Errorf("resourceReadServer(): IPAM provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
			return
		}
		if err = provider.Discover(nodeConfig); err != nil {
			return
		}
		setFlexbotOutput(d, nodeConfig)
	} else {
		d.SetId("")
	}
	return
}

func resourceUpdateServer(d *schema.ResourceData, meta interface{}) (err error) {
	p := meta.(*schema.ResourceData)
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, p); err != nil {
		return
	}
	log.Printf("[INFO] Updating Server %s", nodeConfig.Compute.HostName)
	if d.HasChange("compute") && !d.IsNewResource() {
		oldCompute, newCompute := d.GetChange("compute")
		oldBladeSpec := (oldCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
		newBladeSpec := (newCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
		bladeSpecChange := false
		for _, specItem := range []string{"dn", "model", "num_of_cpus", "num_of_cores", "total_memory"} {
			if oldBladeSpec[specItem].(string) != newBladeSpec[specItem].(string) {
				bladeSpecChange = true
			}
		}
		if bladeSpecChange {
			nodeConfig.Compute.BladeSpec.Dn = ""
		}
		if oldBladeSpec["dn"].(string) != newBladeSpec["dn"].(string) {
			nodeConfig.Compute.BladeSpec.Dn = newBladeSpec["dn"].(string)
			bladeSpecChange = true
		}
		if bladeSpecChange {
			if err = ucsm.UpdateServer(nodeConfig); err == nil {
				if err = ucsm.StartServer(nodeConfig); err == nil {
					err = resourceReadServer(d, meta)
				}
			}
		}
	}
	if d.HasChange("snapshot") && !d.IsNewResource() {
		var oldSnapState, newSnapState, snapStateInter, snapStorage []string
		compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
		sshUser := compute["ssh_user"].(string)
		sshPrivateKey := compute["ssh_private_key"].(string)
		oldSnapshot, newSnapshot := d.GetChange("snapshot")
		for _, snapshot := range oldSnapshot.([]interface{}) {
			oldSnapState = append(oldSnapState, snapshot.(map[string]interface{})["name"].(string))
		}
		for _, snapshot := range newSnapshot.([]interface{}) {
			newSnapState = append(newSnapState, snapshot.(map[string]interface{})["name"].(string))
		}
		snapStateInter = stringSliceIntersection(oldSnapState, newSnapState)
		if snapStorage, err = ontap.GetSnapshots(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(): %s", err)
			return
		}
		for _, name := range oldSnapState {
			if stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
				if err = ontap.DeleteSnapshot(nodeConfig, name); err != nil {
					err = fmt.Errorf("resourceUpdateServer(): %s", err)
					return
				}
			}
		}
		for _, name := range newSnapState {
			if !stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
				for _, snapshot := range newSnapshot.([]interface{}) {
					if snapshot.(map[string]interface{})["name"].(string) == name {
						if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
							if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
								err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
							} else {
								err = fmt.Errorf("expected \"compute.ssh_user\" and \"compute.ssh_private_key\" parameters to ensure fsfreeze for snapshot \"%s\"", name)
							}
						} else {
							err = ontap.CreateSnapshot(nodeConfig, name)
						}
						if err != nil {
							err = fmt.Errorf("resourceUpdateServer(): %s", err)
							return
						}
					}
				}
			}
		}
	}
	return
}

func resourceDeleteServer(d *schema.ResourceData, meta interface{}) (err error) {
	p := meta.(*schema.ResourceData)
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, p); err != nil {
		return
	}
	log.Printf("[INFO] Deleting Server %s", nodeConfig.Compute.HostName)
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	var powerState string
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		err = fmt.Errorf("resourceDeleteServer(): server \"%s\" has power state \"up\"", nodeConfig.Compute.HostName)
		return
	} else {
		if powerState == "up" {
			if err = ucsm.StopServer(nodeConfig); err != nil {
				return
			}
		}
	}
	var stepErr error
	stepErrMsg := []string{}
	stepErr = ucsm.DeleteServer(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	stepErr = ontap.DeleteBootStorage(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	var provider ipam.IpamProvider
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("resourceDeleteServer(): IPAM provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
		return
	}
	stepErr = provider.Release(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	if len(stepErrMsg) > 0 {
		err = fmt.Errorf("resourceDeleteServer(): %s", strings.Join(stepErrMsg, "\n"))
	}
	return
}

func resourceImportServer(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	err := resourceReadServer(d, meta)
	if err != nil {
		return []*schema.ResourceData{}, err
	}
	return []*schema.ResourceData{d}, nil
}

func setFlexbotInput(d *schema.ResourceData, p *schema.ResourceData) (*config.NodeConfig, error) {
	var nodeConfig config.NodeConfig
	var err error
	setInputMutex.Lock()
	defer setInputMutex.Unlock()
	p_ipam := p.Get("ipam").([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.Provider = p_ipam["provider"].(string)
	nodeConfig.Ipam.DnsZone = p_ipam["dns_zone"].(string)
	ibCredentials := p_ipam["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.IbCredentials.Host = ibCredentials["host"].(string)
	nodeConfig.Ipam.IbCredentials.User = ibCredentials["user"].(string)
	nodeConfig.Ipam.IbCredentials.Password = ibCredentials["password"].(string)
	nodeConfig.Ipam.IbCredentials.WapiVersion = ibCredentials["wapi_version"].(string)
	nodeConfig.Ipam.IbCredentials.DnsView = ibCredentials["dns_view"].(string)
	nodeConfig.Ipam.IbCredentials.NetworkView = ibCredentials["network_view"].(string)
	p_compute := p.Get("compute").([]interface{})[0].(map[string]interface{})
	ucsmCredentials := p_compute["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.UcsmCredentials.Host = ucsmCredentials["host"].(string)
	nodeConfig.Compute.UcsmCredentials.User = ucsmCredentials["user"].(string)
	nodeConfig.Compute.UcsmCredentials.Password = ucsmCredentials["password"].(string)
	p_storage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := p_storage["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.CdotCredentials.Host = cdotCredentials["host"].(string)
	nodeConfig.Storage.CdotCredentials.User = cdotCredentials["user"].(string)
	nodeConfig.Storage.CdotCredentials.Password = cdotCredentials["password"].(string)
	nodeConfig.Storage.CdotCredentials.ZapiVersion = cdotCredentials["zapi_version"].(string)
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.SpOrg = compute["sp_org"].(string)
	nodeConfig.Compute.SpTemplate = compute["sp_template"].(string)
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Compute.BladeSpec.Dn = bladeSpec["dn"].(string)
		nodeConfig.Compute.BladeSpec.Model = bladeSpec["model"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCpus = bladeSpec["num_of_cpus"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCores = bladeSpec["num_of_cores"].(string)
		nodeConfig.Compute.BladeSpec.TotalMemory = bladeSpec["total_memory"].(string)
	}
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SvmName = storage["svm_name"].(string)
	nodeConfig.Storage.ImageRepoName = storage["image_repo_name"].(string)
	nodeConfig.Storage.VolumeName = storage["volume_name"].(string)
	nodeConfig.Storage.IgroupName = storage["igroup_name"].(string)
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.BootLun.Name = bootLun["name"].(string)
	nodeConfig.Storage.BootLun.Size = bootLun["size"].(int)
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SeedLun.Name = seedLun["name"].(string)
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Storage.DataLun.Name = dataLun["name"].(string)
		nodeConfig.Storage.DataLun.Size = dataLun["size"].(int)
	}
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	for i, _ := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.Node = append(nodeConfig.Network.Node, config.NetworkInterface{})
		nodeConfig.Network.Node[i].Name = node["name"].(string)
		nodeConfig.Network.Node[i].Ip = node["ip"].(string)
		nodeConfig.Network.Node[i].Fqdn = node["fqdn"].(string)
		nodeConfig.Network.Node[i].Subnet = node["subnet"].(string)
		nodeConfig.Network.Node[i].Gateway = node["gateway"].(string)
		nodeConfig.Network.Node[i].DnsServer1 = node["dns_server1"].(string)
		nodeConfig.Network.Node[i].DnsServer2 = node["dns_server2"].(string)
		nodeConfig.Network.Node[i].DnsDomain = node["dns_domain"].(string)
	}
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.IscsiInitiator = append(nodeConfig.Network.IscsiInitiator, config.IscsiInitiator{})
		nodeConfig.Network.IscsiInitiator[i].Name = initiator["name"].(string)
		nodeConfig.Network.IscsiInitiator[i].Ip = initiator["ip"].(string)
		nodeConfig.Network.IscsiInitiator[i].Fqdn = initiator["fqdn"].(string)
		nodeConfig.Network.IscsiInitiator[i].Subnet = initiator["subnet"].(string)
		nodeConfig.Network.IscsiInitiator[i].Gateway = initiator["gateway"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer1 = initiator["dns_server1"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer2 = initiator["dns_server2"].(string)
		nodeConfig.Network.IscsiInitiator[i].InitiatorName = initiator["initiator_name"].(string)
	}
	nodeConfig.CloudArgs = make(map[string]string)
	for argKey, argValue := range d.Get("cloud_args").(map[string]interface{}) {
		nodeConfig.CloudArgs[argKey] = argValue.(string)
	}
	passPhrase := p.Get("pass_phrase").(string)
	if passPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			return nil, err
		}
	}
	if err = config.SetDefaults(&nodeConfig, compute["hostname"].(string), bootLun["os_image"].(string), seedLun["seed_template"].(string), passPhrase); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	}
	return &nodeConfig, err
}

func setFlexbotOutput(d *schema.ResourceData, nodeConfig *config.NodeConfig) {
	setOutputMutex.Lock()
	defer setOutputMutex.Unlock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	compute["sp_dn"] = nodeConfig.Compute.SpDn
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		bladeSpec["dn"] = nodeConfig.Compute.BladeSpec.Dn
		compute["blade_spec"].([]interface{})[0] = bladeSpec
	}
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	storage["svm_name"] = nodeConfig.Storage.SvmName
	storage["image_repo_name"] = nodeConfig.Storage.ImageRepoName
	storage["volume_name"] = nodeConfig.Storage.VolumeName
	storage["igroup_name"] = nodeConfig.Storage.IgroupName
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	bootLun["name"] = nodeConfig.Storage.BootLun.Name
	bootLun["id"] = nodeConfig.Storage.BootLun.Id
	storage["boot_lun"].([]interface{})[0] = bootLun
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	seedLun["name"] = nodeConfig.Storage.SeedLun.Name
	seedLun["id"] = nodeConfig.Storage.SeedLun.Id
	storage["seed_lun"].([]interface{})[0] = seedLun
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		dataLun["name"] = nodeConfig.Storage.DataLun.Name
		dataLun["id"] = nodeConfig.Storage.DataLun.Id
		storage["data_lun"].([]interface{})[0] = dataLun
	}
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	for i, _ := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		node["macaddr"] = nodeConfig.Network.Node[i].Macaddr
		node["ip"] = nodeConfig.Network.Node[i].Ip
		node["fqdn"] = nodeConfig.Network.Node[i].Fqdn
		network["node"].([]interface{})[i] = node
	}
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		initiator["ip"] = nodeConfig.Network.IscsiInitiator[i].Ip
		initiator["initiator_name"] = nodeConfig.Network.IscsiInitiator[i].InitiatorName
		initiator["fqdn"] = nodeConfig.Network.IscsiInitiator[i].Fqdn
		initiator["subnet"] = nodeConfig.Network.IscsiInitiator[i].Subnet
		initiator["gateway"] = nodeConfig.Network.IscsiInitiator[i].Gateway
		initiator["dns_server1"] = nodeConfig.Network.IscsiInitiator[i].DnsServer1
		initiator["dns_server2"] = nodeConfig.Network.IscsiInitiator[i].DnsServer2
		if len(initiator["iscsi_target"].([]interface{})) == 0 {
			if nodeConfig.Network.IscsiInitiator[i].IscsiTarget != nil {
				iscsi_target := make(map[string]interface{})
				iscsi_target["node_name"] = nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName
				iscsi_target["interfaces"] = []string{}
				for _, iface := range nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces {
					iscsi_target["interfaces"] = append(iscsi_target["interfaces"].([]string), iface)
				}
				initiator["iscsi_target"] = append(initiator["iscsi_target"].([]interface{}), iscsi_target)
			}
		}
		network["iscsi_initiator"].([]interface{})[i] = initiator
	}
	d.Set("compute", []interface{}{compute})
	d.Set("storage", []interface{}{storage})
	d.Set("network", []interface{}{network})
}

func checkSshListen(host string) (listen bool) {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", host+":22", timeout)
	if err != nil {
		listen = false
	} else {
		listen = true
		conn.Close()
	}
	return
}

func checkSshCommand(host string, sshUser string, sshPrivateKey string) (err error) {
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session

	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", host + ":22", config); err != nil {
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		return
	}
	err = sess.Run("uname -a")
	sess.Close()
	return
}

func createSnapshot(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string, snapshotName string) (err error) {
	var filesystems, cmd, errs []string
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var b_stdout, b_stderr bytes.Buffer

	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", nodeConfig.Network.Node[0].Ip + ":22", config); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to connect to host %s: %s", nodeConfig.Network.Node[0].Ip, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	err = sess.Run(`cat /proc/mounts | sed -n 's/^\/dev\/mapper\/[^ ]\+[ ]\+\(\/[^ ]\{1,64\}\).*/\1/p'`)
	sess.Close()
	if err != nil {
		err = fmt.Errorf("createSnapshot(): failed to run command: %s: %s", err, b_stderr.String())
		return
	}
	if b_stdout.Len() > 0 {
		filesystems = strings.Split(strings.Trim(b_stdout.String(), "\n"), "\n")
	}
	for _, fs := range filesystems {
		cmd = append(cmd, "fsfreeze -f " + fs)
	}
	cmd = append(cmd, "fsfreeze -f / && echo -n frozen && sleep 5 && fsfreeze -u /")
	for _, fs := range filesystems {
		cmd = append(cmd, "fsfreeze -u " + fs)
	}
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	b_stdout.Reset()
	b_stderr.Reset()
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	if err = sess.Start(fmt.Sprintf(`sudo -n sh -c '%s'`, strings.Join(cmd, " && "))); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to start SSH command: %s", err)
		return
	}
	for i := 0; i < 10; i++ {
		if b_stdout.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if b_stdout.String() == "frozen" {
		if err = ontap.CreateSnapshot(nodeConfig, snapshotName); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err = sess.Wait(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to run SSH command: %s: %s", err, b_stderr.String()))
	}
	if err != nil {
		err = fmt.Errorf("createSnapshot(): %s", strings.Join(errs, " , "))
	}
	return
}

func stringSliceIntersection(src1, src2 []string) (dst []string) {
	hash := make(map[string]bool)
	for _, e := range src1 {
		hash[e] = true
	}
	for _, e := range src2 {
		if hash[e] {
			dst = append(dst, e)
		}
	}
	return
}

func stringSliceElementExists(array []string, elem string) (bool) {
	for _, e := range array {
		if e == elem {
			return true
		}
	}
	return false
}
