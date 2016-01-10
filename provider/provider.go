package provider

import "github.com/LunaNode/cloug/provider/digitalocean"
import "github.com/LunaNode/cloug/provider/lunanode"
import "github.com/LunaNode/cloug/provider/openstack"
import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"
import "fmt"

type ComputeConfig struct {
	// one of solusvm, openstack, cloudstack, lobster, lndynamic, fake, digitalocean, vultr, linode
	Provider string `json:"provider"`

	// API options (used by solusvm, cloudstack, lobster, lndynamic, digitalocean, vultr, linode)
	ApiID  string `json:"api_id"`
	ApiKey string `json:"api_key"`

	// URL (used by solusvm, lobster, openstack, cloudstack)
	URL string `json:"url"`

	// solusvm options
	VirtType  string `json:"virt_type"`
	NodeGroup string `json:"node_group"`
	Insecure  bool   `json:"insecure"`

	// openstack options
	Username string `json:"username"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`

	// cloudstack options
	SecretKey string `json:"secret_key"`
	ZoneID    string `json:"zone_id"`
}

func ComputeProviderFromConfig(cfg *ComputeConfig) (compute.Provider, error) {
	if cfg.Provider == "openstack" {
		return openstack.MakeOpenStack(cfg.URL, cfg.Username, cfg.Password, cfg.Tenant)
	} else if cfg.Provider == "lunanode" {
		return lunanode.MakeLunaNode(cfg.ApiID, cfg.ApiKey)
	} else if cfg.Provider == "digitalocean" {
		return digitalocean.MakeDigitalOcean(cfg.ApiID), nil
	} else {
		return nil, fmt.Errorf("invalid provider type %s", cfg.Provider)
	}
}

func ComputeProviderFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg ComputeConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return ComputeProviderFromConfig(&cfg)
}
