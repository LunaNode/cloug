package provider

import "github.com/LunaNode/cloug/provider/cloudstack"
import "github.com/LunaNode/cloug/provider/digitalocean"
import "github.com/LunaNode/cloug/provider/ec2"
import "github.com/LunaNode/cloug/provider/googlecompute"
import "github.com/LunaNode/cloug/provider/linode"
import "github.com/LunaNode/cloug/provider/lobster"
import "github.com/LunaNode/cloug/provider/lunanode"
import "github.com/LunaNode/cloug/provider/openstack"
import "github.com/LunaNode/cloug/provider/proxmox"
import "github.com/LunaNode/cloug/provider/solusvm"
import "github.com/LunaNode/cloug/provider/vultr"
import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"
import "fmt"

type ProviderJSONFunc func(jsonData []byte) (compute.Provider, error)

var providerJSONFuncs map[string]ProviderJSONFunc = map[string]ProviderJSONFunc{
	"openstack":     openstack.OpenStackFromJSON,
	"cloudstack":    cloudstack.CloudStackFromJSON,
	"proxmox":       proxmox.ProxmoxFromJSON,
	"solusvm":       solusvm.SolusVMFromJSON,
	"lunanode":      lunanode.LunaNodeFromJSON,
	"lobster":       lobster.LobsterFromJSON,
	"ec2":           ec2.EC2FromJSON,
	"googlecompute": googlecompute.GoogleComputeFromJSON,
	"digitalocean":  digitalocean.DigitalOceanFromJSON,
	"linode":        linode.LinodeFromJSON,
	"vultr":         vultr.VultrFromJSON,
}

type ComputeConfig struct {
	Provider string `json:"provider"`
}

func ComputeProviderFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg ComputeConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	} else if providerJSONFuncs[cfg.Provider] == nil {
		return nil, fmt.Errorf("invalid provider type %s", cfg.Provider)
	}
	return providerJSONFuncs[cfg.Provider](jsonData)
}
