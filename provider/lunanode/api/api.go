package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type API struct {
	ApiId         string
	ApiKey        string
	ApiPartialKey string
}

func MakeAPI(id string, key string) (*API, error) {
	if len(id) != 16 {
		return nil, fmt.Errorf("API ID length must be 16, but parameter has length %d", len(id))
	} else if len(key) != 128 {
		return nil, fmt.Errorf("API key length must be 128, but parameter has length %d", len(key))
	}

	api := new(API)
	api.ApiId = id
	api.ApiKey = key
	api.ApiPartialKey = key[:64]
	return api, nil
}

func (api *API) request(category string, action string, params map[string]string, target interface{}) error {
	// construct URL
	targetUrl := LNDYNAMIC_API_URL
	targetUrl = strings.Replace(targetUrl, "{CATEGORY}", category, -1)
	targetUrl = strings.Replace(targetUrl, "{ACTION}", action, -1)

	// get raw parameters string
	if params == nil {
		params = make(map[string]string)
	}
	params["api_id"] = api.ApiId
	params["api_partialkey"] = api.ApiPartialKey
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}

	// HMAC signature with nonce
	nonce := fmt.Sprintf("%d", time.Now().Unix())
	handler := fmt.Sprintf("%s/%s/", category, action)
	hashTarget := fmt.Sprintf("%s|%s|%s", handler, string(rawParams), nonce)
	hasher := hmac.New(sha512.New, []byte(api.ApiKey))
	if _, err := hasher.Write([]byte(hashTarget)); err != nil {
		return err
	}
	signature := hex.EncodeToString(hasher.Sum(nil))

	// perform request
	values := url.Values{}
	values.Set("handler", handler)
	values.Set("req", string(rawParams))
	values.Set("signature", signature)
	values.Set("nonce", nonce)
	byteBuffer := new(bytes.Buffer)
	byteBuffer.Write([]byte(values.Encode()))
	response, err := http.Post(targetUrl, "application/x-www-form-urlencoded", byteBuffer)
	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body.Close()

	// decode JSON
	// we first decode into generic response for error checking; then into specific response to return
	var genericResponse GenericResponse
	if err := json.Unmarshal(responseBytes, &genericResponse); err != nil {
		return err
	} else if genericResponse.Success != "yes" {
		if genericResponse.Error != "" {
			return errors.New(genericResponse.Error)
		} else {
			return errors.New("backend call failed for unknown reason")
		}
	}

	if target != nil {
		if err := json.Unmarshal(responseBytes, target); err != nil {
			return err
		}
	}

	return nil
}

func (api *API) uid() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

// virtual machines

func (api *API) VmCreateVolume(region string, hostname string, planIdentification int, volumeIdentification int) (string, error) {
	params := make(map[string]string)
	params["hostname"] = hostname
	params["region"] = region
	params["plan_id"] = fmt.Sprintf("%d", planIdentification)
	params["volume_id"] = fmt.Sprintf("%d", volumeIdentification)
	var response VmCreateResponse
	if err := api.request("vm", "create", params, &response); err != nil {
		return "", err
	} else {
		return response.ID, nil
	}
}
func (api *API) VmCreateImage(region string, hostname string, planIdentification int, imageIdentification int) (string, error) {
	params := make(map[string]string)
	params["hostname"] = hostname
	params["region"] = region
	params["plan_id"] = fmt.Sprintf("%d", planIdentification)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	var response VmCreateResponse
	if err := api.request("vm", "create", params, &response); err != nil {
		return "", err
	} else {
		return response.ID, nil
	}
}

func (api *API) vmAction(vmIdentification string, action string, params map[string]string) error {
	if params == nil {
		params = make(map[string]string)
	}
	params["vm_id"] = vmIdentification
	return api.request("vm", action, params, nil)
}

func (api *API) VmStart(vmIdentification string) error {
	return api.vmAction(vmIdentification, "start", nil)
}

func (api *API) VmStop(vmIdentification string) error {
	return api.vmAction(vmIdentification, "stop", nil)
}

func (api *API) VmReboot(vmIdentification string) error {
	return api.vmAction(vmIdentification, "reboot", nil)
}

func (api *API) VmDelete(vmIdentification string) error {
	return api.vmAction(vmIdentification, "delete", nil)
}

func (api *API) VmDiskSwap(vmIdentification string) error {
	return api.vmAction(vmIdentification, "diskswap", nil)
}

func (api *API) VmReimage(vmIdentification string, imageIdentification int) error {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	return api.vmAction(vmIdentification, "reimage", params)
}

func (api *API) VmVnc(vmIdentification string) (string, error) {
	params := make(map[string]string)
	params["vm_id"] = vmIdentification
	var response VmVncResponse
	if err := api.request("vm", "vnc", params, &response); err != nil {
		return "", err
	} else {
		return response.VncUrl, nil
	}
}

func (api *API) VmList() ([]VmStruct, error) {
	var response VmListResponse
	if err := api.request("vm", "list", nil, &response); err != nil {
		return nil, err
	} else {
		return response.Vms, nil
	}
}

func (api *API) VmInfo(vmIdentification string) (*VmStruct, *VmInfoStruct, error) {
	params := make(map[string]string)
	params["vm_id"] = vmIdentification
	var response VmInfoResponse
	if err := api.request("vm", "info", params, &response); err != nil {
		return nil, nil, err
	} else {
		return response.Extra, response.Info, nil
	}
}

func (api *API) VmSnapshot(vmIdentification string) (int, error) {
	// create snapshot with random label
	imageLabel := api.uid()
	params := make(map[string]string)
	params["vm_id"] = vmIdentification
	params["name"] = imageLabel
	var response ImageCreateResponse
	if err := api.request("vm", "snapshot", params, &response); err != nil {
		return 0, err
	} else {
		return response.ID, nil
	}
}

// images

func (api *API) ImageFetch(region string, location string, format string, virtio bool) (int, error) {
	// create an image with random label
	imageLabel := api.uid()
	params := make(map[string]string)
	params["region"] = region
	params["name"] = imageLabel
	params["location"] = location
	params["format"] = format
	if virtio {
		params["virtio"] = "yes"
	}
	var response ImageCreateResponse
	if err := api.request("image", "fetch", params, &response); err != nil {
		return 0, err
	} else {
		return response.ID, nil
	}
}

func (api *API) ImageDetails(imageIdentification int) (*Image, error) {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	var response ImageDetailsResponse
	if err := api.request("image", "details", params, &response); err != nil {
		return nil, err
	} else {
		return response.Image, nil
	}
}

func (api *API) ImageDelete(imageIdentification int) error {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	return api.request("image", "delete", params, nil)
}

func (api *API) ImageList(region string) ([]*Image, error) {
	params := make(map[string]string)
	if region != "" {
		params["region"] = region
	}
	var listResponse ImageListResponse
	if err := api.request("image", "list", params, &listResponse); err != nil {
		return nil, err
	} else {
		return listResponse.Images, nil
	}
}

// volumes

// Create a volume with the given size in gigabytes and image identification (or nil for empty volume).
func (api *API) VolumeCreate(region string, size int, imageIdentification *int) (int, error) {
	// create a volume with random label
	volumeLabel := api.uid()
	params := make(map[string]string)
	params["region"] = region
	params["label"] = volumeLabel
	params["size"] = fmt.Sprintf("%d", size)

	if imageIdentification != nil {
		params["image"] = fmt.Sprintf("%d", *imageIdentification)
	}

	var response VolumeCreateResponse
	if err := api.request("volume", "create", params, &response); err != nil {
		return 0, err
	} else {
		return response.ID, nil
	}
}

func (api *API) VolumeDelete(region string, volumeIdentification int) error {
	params := make(map[string]string)
	params["region"] = region
	params["volume_id"] = fmt.Sprintf("%d", volumeIdentification)
	return api.request("volume", "delete", params, nil)
}

func (api *API) VolumeInfo(region string, volumeIdentification int) (*Volume, error) {
	var response VolumeInfoResponse
	err := api.request("volume", "info", map[string]string{
		"region":    region,
		"volume_id": strconv.Itoa(volumeIdentification),
	}, &response)
	if err != nil {
		return nil, err
	} else {
		return response.Volume, nil
	}
}

func (api *API) VolumeList(region string) ([]*Volume, error) {
	var response VolumeListResponse
	err := api.request("volume", "list", map[string]string{
		"region": region,
	}, &response)
	if err != nil {
		return nil, err
	} else {
		return response.Volumes, nil
	}
}

func (api *API) VolumeAttach(region string, volumeIdentification int, vmIdentification string) error {
	return api.request("volume", "attach", map[string]string{
		"region":    region,
		"volume_id": strconv.Itoa(volumeIdentification),
		"vm_id":     vmIdentification,
		"target":    "auto",
	}, nil)
}

func (api *API) VolumeDetach(region string, volumeIdentification int) error {
	return api.request("volume", "detach", map[string]string{
		"region":    region,
		"volume_id": strconv.Itoa(volumeIdentification),
	}, nil)
}

// plans

func (api *API) PlanList() ([]*Plan, error) {
	var listResponse PlanListResponse
	err := api.request("plan", "list", nil, &listResponse)
	return listResponse.Plans, err
}
