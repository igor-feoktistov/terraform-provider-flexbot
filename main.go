package main

import (
	"github.com/igor-feoktistov/terraform-provider-flexbot/flexbot"
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: flexbot.Provider,
	})
}
