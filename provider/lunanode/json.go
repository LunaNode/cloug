package lunanode

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type LunaNodeJSONConfig struct {
	ApiID  string `json:"api_id"`
	ApiKey string `json:"api_key"`
}

func LunaNodeFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg LunaNodeJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeLunaNode(cfg.ApiID, cfg.ApiKey)
}
