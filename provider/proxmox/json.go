package proxmox

import "github.com/LunaNode/cloug/service/compute"

import "crypto/tls"
import "encoding/json"
import "net/http"

type ProxmoxJSONConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

func ProxmoxFromJSON(jsonData []byte) (compute.Provider, error) {
	var cfg ProxmoxJSONConfig
	err := json.Unmarshal(jsonData, &cfg)
	if err != nil {
		return nil, err
	}
	proxmox, err := MakeProxmox(cfg.URL, cfg.Username, cfg.Password), nil
	if err != nil {
		return nil, err
	}

	if cfg.Insecure {
		proxmox.Client.Client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	return proxmox, nil
}
