package googlecompute

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type GoogleComputeJSONConfig struct {
	Email      string `json:"email"`
	PrivateKey string `json:"private_key"`
	Project    string `json:"project"`
}

func GoogleComputeFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg GoogleComputeJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeGoogleCompute(cfg.Email, cfg.PrivateKey, cfg.Project)
}
