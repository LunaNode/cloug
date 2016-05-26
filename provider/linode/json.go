package linode

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type LinodeJSONConfig struct {
	ApiKey string `json:"api_key"`
}

func LinodeFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg LinodeJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeLinode(cfg.ApiKey), nil
}
