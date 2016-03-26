package lobster

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type LobsterJSONConfig struct {
	URL    string `json:"url"`
	ApiID  string `json:"api_id"`
	ApiKey string `json:"api_key"`
}

func LobsterFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg LobsterJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeLobster(cfg.URL, cfg.ApiID, cfg.ApiKey), nil
}
