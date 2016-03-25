package ec2

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type EC2JSONConfig struct {
	KeyID     string `json:"key_id"`
	SecretKey string `json:"secret_key"`
	Token     string `json:"token"`
}

func EC2FromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg EC2JSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return MakeEC2(cfg.KeyID, cfg.SecretKey, cfg.Token)
}
