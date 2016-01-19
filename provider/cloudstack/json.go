package cloudstack

import "encoding/json"

type CloudStackJSONConfig struct {
	URL       string `json:"url"`
	ZoneID    string `json:"zone_id"`
	NetworkID string `json:"network_id"`
	ApiKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

func CloudStackFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg CloudStackJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeCloudStack(cfg.URL, cfg.ZoneID, cfg.NetworkID, cfg.ApiKey, cfg.SecretKey), nil
}
