package digitalocean

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type DigitalOceanJSONConfig struct {
	Token string `json:"token"`
}

func DigitalOceanFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg DigitalOceanJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeDigitalOcean(cfg.Token), nil
}
