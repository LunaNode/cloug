package solusvm

import "github.com/LunaNode/cloug/service/compute"

import "encoding/json"

type SolusJSONConfig struct {
	URL       string `json:"url"`
	ApiID     string `json:"api_id"`
	ApiKey    string `json:"api_key"`
	Insecure  bool   `json:"insecure"`
	VirtType  string `json:"virt_type"`
	NodeGroup string `json:"node_group"`
}

func SolusVMFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg SolusJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	return &SolusVM{
		VirtType: cfg.VirtType,
		NodeGroup: cfg.NodeGroup,
		Api: &API{
			Url: cfg.URL,
			ApiId: cfg.ApiID,
			ApiKey: cfg.ApiKey,
			Insecure: cfg.Insecure,
		},
	}, nil
}
