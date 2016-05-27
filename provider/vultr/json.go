package vultr

import "github.com/LunaNode/cloug/service/compute"

import vultr "github.com/LunaNode/vultr/lib"

import "encoding/json"

type VultrJSONConfig struct {
	ApiKey string `json:"api_key"`
}

func VultrFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg VultrJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return &Vultr{
		client: vultr.NewClient(cfg.ApiKey, nil),
	}, nil
}
