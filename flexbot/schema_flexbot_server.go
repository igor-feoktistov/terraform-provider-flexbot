package flexbot

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var (
        // Available maintenance tasks
        MaintenanceTasks = []string{"cordon", "uncordon", "drain", "restart"}
        // Node taint effects
        TaintEffects = []string{"PreferNoSchedule", "NoSchedule", "NoExecute"}
)

// Schemas
func schemaFlexbotServer() map[string]*schema.Schema {
	return map[string]*schema.Schema{
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
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
						Default:   "",
					},
					"ssh_node_init_commands": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"ssh_node_bootdisk_resize_commands": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"ssh_node_datadisk_resize_commands": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
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
								"num_of_threads": {
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
					"blade_assigned": {
						Type:     schema.TypeList,
						Optional: true,
						Computed: true,
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
									Computed: true,
								},
								"serial": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"num_of_cpus": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"num_of_cores": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"num_of_threads": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"total_memory": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
							},
						},
					},
					"powerstate": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "up",
						ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
							v := val.(string)
							if !(v == "up" || v == "down") {
								errs = append(errs, fmt.Errorf("value %q=%s must be either \"up\" or \"down\"", key, v))
							}
							return
						},
					},
					"description": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "",
					},
					"label": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "",
					},
				},
			},
		},
		"storage": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"svm_name": {
						Type:     schema.TypeString,
						Optional: true,
						Computed: true,
						ForceNew: true,
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
										if v < 0 || v > 4096 {
											errs = append(errs, fmt.Errorf("%q must be between 0 and 4096 inclusive, got: %d", key, v))
										}
										return
									},
								},
							},
						},
					},
					"data_nvme": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"namespace": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"subsystem": {
									Type:     schema.TypeString,
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
					"auto_snapshot_on_update": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"force_update": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"snapshots": {
						Type:     schema.TypeList,
						Optional: true,
						Computed: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
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
										if len(v) > 0 {
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("value %q=%s must be in IP address format", key, v))
											}
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
								"ip_range": {
									Type:     schema.TypeString,
									Optional: true,
									ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
										v := val.(string)
										if len(v) > 0 {
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\s*-\s*\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("unexpected IP range format: %q=%s", key, v))
											}
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
								"dns_server3": {
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
								"parameters": {
									Type:     schema.TypeMap,
									Optional: true,
									Default:  make(map[string]interface{}),
									Elem:     &schema.Schema{Type: schema.TypeString},
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
								"ip_range": {
									Type:     schema.TypeString,
									Optional: true,
									ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
										v := val.(string)
										if len(v) > 0 {
											matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\s*-\s*\d+\.\d+\.\d+\.\d+$`, v)
											if !matched {
												errs = append(errs, fmt.Errorf("unexpected IP range format: %q=%s", key, v))
											}
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
								"parameters": {
									Type:     schema.TypeMap,
									Optional: true,
									Elem:     &schema.Schema{Type: schema.TypeString},
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
					"nvme_host": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: 2,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"host_interface": {
									Type:     schema.TypeString,
									Required: true,
								},
								"ip": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"subnet": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"host_nqn": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"nvme_target": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"target_nqn": {
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
		"labels": {
			Type:     schema.TypeMap,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"taints": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:     schema.TypeString,
						Required: true,
					},
					"value": {
						Type:     schema.TypeString,
						Required: true,
					},
					"effect": {
						Type:     schema.TypeString,
						Required: true,
						ValidateFunc: validation.StringInSlice(TaintEffects, true),
					},
				},
			},
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
		"restore": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"restore": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"snapshot_name": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "",
					},
				},
			},
		},
		"maintenance": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"execute": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"synchronized_run": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
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
					"tasks": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{
						        Type: schema.TypeString,
						        ValidateFunc: validation.StringInSlice(MaintenanceTasks, true),
						},
					},
				},
			},
		},
	}
}
