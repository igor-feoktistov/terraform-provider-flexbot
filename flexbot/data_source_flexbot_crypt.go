package flexbot

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
)

func dataSourceFelxbotCrypt() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceFlexbotCryptRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Entity string name",
			},
			"encrypted": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Encrypted string encoded in base64 format",
			},
			"decrypted": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Description: "Decrypted sting in plain text",
			},
		},
	}
}

func dataSourceFlexbotCryptRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	var err error
	var b, b64 []byte
	passPhrase := meta.(*FlexbotConfig).FlexbotProvider.Get("pass_phrase").(string)
	name := d.Get("name").(string)
	encrypted := d.Get("encrypted").(string)
	decrypted := d.Get("decrypted").(string)
	if len(encrypted) == 0 && len(decrypted) > 0 {
		if b, err = crypt.Encrypt([]byte(decrypted), passPhrase); err != nil {
			diags = diag.FromErr(fmt.Errorf("dataSourceFlexbotCryptRead(): crypt.Encrypt() error: %s", err))
			return
		}
		d.Set("encrypted", "base64:"+base64.StdEncoding.EncodeToString(b))
	}
	if len(decrypted) == 0 && len(encrypted) > 0 {
		if b64, err = base64.StdEncoding.DecodeString(encrypted[7:]); err != nil {
			diags = diag.FromErr(fmt.Errorf("dataSourceFlexbotCryptRead(): base64.StdEncoding.DecodeString() failure: %s", err))
			return
		}
		if b, err = crypt.Decrypt(b64, passPhrase); err != nil {
			diags = diag.FromErr(fmt.Errorf("dataSourceFlexbotCryptRead(): crypt.Decrypt() error: %s", err))
			return
		}
		d.Set("decrypted", string(b))
	}
	d.SetId(name)
	return
}
