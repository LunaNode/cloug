package openstack

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type OpenStackJSONConfig struct {
	URL        string `json:"url"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	TenantName string `json:"tenant"`
}

func OpenStackFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg OpenStackJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeOpenStack(cfg.URL, cfg.Username, cfg.Password, cfg.TenantName)
}
