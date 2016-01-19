package api

import "bytes"
import "encoding/json"
import "errors"
import "fmt"
import "io"
import "io/ioutil"
import "net/url"
import "net/http"
import "strconv"
import "sync"

type API struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client

	ticket              string
	csrfPreventionToken string
	mu                  sync.Mutex
}

func NewAPI(baseURL string, username string, password string) *API {
	return &API{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   &http.Client{},
	}
}

func (api *API) request(method string, path string, params map[string]string, response interface{}, setAuthParams bool) error {
	var body io.Reader
	if params != nil {
		values := make(url.Values)
		for k, v := range params {
			values.Set(k, v)
		}
		body = bytes.NewBufferString(values.Encode())
	}
	httpRequest, err := http.NewRequest(method, api.BaseURL+path, body)
	httpRequest.Header.Set("Accept", "application/json")

	if setAuthParams {
		ticket, csrfPreventionToken, err := api.getAuthParams()
		if err != nil {
			return err
		}
		httpRequest.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", ticket))
		httpRequest.Header.Set("CSRFPreventionToken", csrfPreventionToken)
	}

	r, err := api.Client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("http request error: %v", err)
	}
	responseBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("http read error: %v", err)
	}

	// extract JSON string inside data field
	decoder := json.NewDecoder(bytes.NewReader(responseBytes))
	decoder.UseNumber()
	jsonMap := make(map[string]interface{})
	err = decoder.Decode(&jsonMap)
	if err != nil {
		return fmt.Errorf("json decode error: %v", err)
	} else if _, ok := jsonMap["data"]; !ok {
		return errors.New("API response does not contain data key")
	}
	dataBytes, err := json.Marshal(jsonMap["data"])
	if err != nil {
		return fmt.Errorf("json error: %v", err)
	}

	if r.StatusCode >= 200 && r.StatusCode < 300 {
		if response != nil {
			err = json.Unmarshal(dataBytes, response)
			if err != nil {
				return fmt.Errorf("json decode error: %v", err)
			}
		}
	} else {
		if jsonMap["data"] == nil {
			return errors.New("API error")
		} else {
			return errors.New(string(dataBytes))
		}
	}

	return nil
}

func (api *API) getAuthParams() (string, string, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.ticket == "" || api.csrfPreventionToken == "" {
		err := api.authenticate()
		if err != nil {
			return "", "", err
		}
	}
	return api.ticket, api.csrfPreventionToken, nil
}

// Gets the authentication parameters for Proxmox API.
// Assumes caller has the api.mu lock.
func (api *API) authenticate() error {
	var response AuthenticateResponse
	err := api.request("POST", "/access/ticket", map[string]string{
		"username": api.Username,
		"password": api.Password,
	}, &response, false)
	if err != nil {
		return err
	}

	api.ticket = response.Ticket
	api.csrfPreventionToken = response.CSRFPreventionToken
	return nil
}

func (api *API) get(path string, response interface{}) error {
	return api.request("GET", path, nil, response, true)
}

func (api *API) post(path string, params map[string]string, response interface{}) error {
	return api.request("POST", path, params, response, true)
}

func (api *API) del(path string, params map[string]string, response interface{}) error {
	return api.request("DELETE", path, params, response, true)
}

func (api *API) ListNodes() ([]Node, error) {
	var nodes []Node
	err := api.get("/nodes", &nodes)
	if err != nil {
		return nil, err
	} else {
		return nodes, nil
	}
}

func (api *API) ListVMsOnNode(node string) ([]VM, error) {
	var vms []VM
	err := api.get(fmt.Sprintf("/nodes/%s/qemu", node), &vms)
	if err != nil {
		return nil, err
	} else {
		return vms, nil
	}
}

func (api *API) CreateVM(options *CreateVMOptions) error {
	params := map[string]string{
		"vmid":   strconv.Itoa(options.ID),
		"name":   options.Name,
		"cores":  strconv.Itoa(options.Cores),
		"memory": strconv.Itoa(options.Memory),
		"ostype": string(options.OSType),
		"ide0":   fmt.Sprintf("volume=%s:%d", options.Storage, options.DiskSize),
		"cdrom":  fmt.Sprintf("%s,media=cdrom", options.ISO),
	}

	// network settings
	networkDriver := string(options.NetworkDriver)
	if networkDriver == "" {
		networkDriver = "e1000"
	}
	if options.NetworkBridge != "" {
		params["net0"] = fmt.Sprintf("%s,bridge=%s", networkDriver, options.NetworkBridge)
	} else {
		params["net0"] = networkDriver
	}

	return api.post(fmt.Sprintf("/nodes/%s/qemu", options.Node), params, nil)
}

func (api *API) DeleteVM(node string, id int) error {
	return api.del(fmt.Sprintf("nodes/%s/qemu/%d", node, id), nil, nil)
}

func (api *API) GetVMStatus(node string, id int) (*VM, error) {
	var vm VM
	err := api.get(fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, id), &vm)
	if err != nil {
		return nil, err
	} else {
		return &vm, nil
	}
}

func (api *API) StartVM(node string, id int) error {
	return api.post(fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, id), nil, nil)
}

func (api *API) StopVM(node string, id int) error {
	return api.post(fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, id), nil, nil)
}

func (api *API) ResetVM(node string, id int) error {
	return api.post(fmt.Sprintf("/nodes/%s/qemu/%d/status/reset", node, id), nil, nil)
}

func (api *API) VNCProxy(node string, id int) error {
	return api.post(fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", node, id), nil, nil)
}
