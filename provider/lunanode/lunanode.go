package lunanode

import "github.com/LunaNode/cloug/provider/common"
import lnapi "github.com/LunaNode/cloug/provider/lunanode/api"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "errors"
import "fmt"
import "strconv"
import "strings"

const DEFAULT_NAME = "cloug"
const DEFAULT_REGION = "toronto"

type LunaNode struct {
	api *lnapi.API
}

func MakeLunaNode(apiId string, apiKey string) (*LunaNode, error) {
	api, err := lnapi.MakeAPI(apiId, apiKey)
	if err != nil {
		return nil, err
	} else {
		return &LunaNode{api}, nil
	}
}

func (ln *LunaNode) ComputeService() compute.Service {
	return ln
}

func (ln *LunaNode) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "Online" {
		return compute.StatusOnline
	} else if status == "Offline" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (ln *LunaNode) vmToInstance(vm *lnapi.VmStruct, info *lnapi.VmInfoStruct) *compute.Instance {
	instance := &compute.Instance{
		ID:        vm.ID,
		Name:      vm.Name,
		Region:    vm.Region,
		IP:        vm.PrimaryIP,
		PrivateIP: vm.PrivateIP,
		Flavor:    compute.Flavor{ID: vm.PlanID},
	}

	if info != nil {
		instance.IP = info.IP
		instance.Status = ln.mapInstanceStatus(info.Status)
		bwFloat, _ := strconv.ParseFloat(info.BandwidthUsed, 64)
		instance.BandwidthUsed = int64(bwFloat * 1024 * 1024 * 1024)

		// parse login details, should be one of:
		//  "username: ABC; password: XYZ"
		//  "password: XYZ"
		for _, loginPart := range strings.Split(info.LoginDetails, "; ") {
			parts := strings.Split(loginPart, ": ")
			if len(parts) == 2 {
				if parts[0] == "username" {
					instance.Username = parts[1]
				} else if parts[0] == "password" {
					instance.Password = parts[1]
				}
			}
		}
	}

	return instance
}

func (ln *LunaNode) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(ln, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(ln, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, fmt.Errorf("invalid image ID: %s", imageID)
	}
	flavorIDInt, err := strconv.Atoi(flavorID)
	if err != nil {
		return nil, fmt.Errorf("invalid flavor ID: %s", flavorID)
	}

	region := instance.Region
	if region == "" {
		region = DEFAULT_REGION
	}

	name := instance.Name
	if name == "" {
		name = DEFAULT_NAME
	}

	vmId, err := ln.api.VmCreateImage(region, name, flavorIDInt, imageIDInt)
	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID: fmt.Sprintf("%d", vmId),
		}, nil
	}
}

func (ln *LunaNode) instanceAction(instanceID string, f func(id int) error) error {
	instanceIDInt, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %s", instanceID)
	} else {
		return f(instanceIDInt)
	}
}

func (ln *LunaNode) DeleteInstance(instanceID string) error {
	return ln.instanceAction(instanceID, ln.api.VmDelete)
}

func (ln *LunaNode) ListInstances() ([]*compute.Instance, error) {
	vms, err := ln.api.VmList()
	if err != nil {
		return nil, err
	}
	instances := make([]*compute.Instance, len(vms))
	for i, vm := range vms {
		instances[i] = ln.vmToInstance(&vm, nil)
	}
	return instances, nil
}

func (ln *LunaNode) GetInstance(instanceID string) (*compute.Instance, error) {
	instanceIDInt, _ := strconv.Atoi(instanceID)
	vm, info, err := ln.api.VmInfo(instanceIDInt)
	if err != nil {
		return nil, err
	} else {
		return ln.vmToInstance(vm, info), nil
	}
}

func (ln *LunaNode) StartInstance(instanceID string) error {
	return ln.instanceAction(instanceID, ln.api.VmStart)
}

func (ln *LunaNode) StopInstance(instanceID string) error {
	return ln.instanceAction(instanceID, ln.api.VmStop)
}

func (ln *LunaNode) RebootInstance(instanceID string) error {
	return ln.instanceAction(instanceID, ln.api.VmReboot)
}

func (ln *LunaNode) GetVNC(instanceID string) (string, error) {
	var url string
	err := ln.instanceAction(instanceID, func(id int) error {
		var err error
		url, err = ln.api.VmVnc(id)
		return err
	})
	return url, err
}

func (ln *LunaNode) ReimageInstance(instanceID string, image *compute.Image) error {
	imageID, err := common.GetMatchingImageID(ln, image)
	if err != nil {
		return err
	}
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return fmt.Errorf("invalid image ID: %s", imageID)
	}

	return ln.instanceAction(instanceID, func(id int) error {
		return ln.api.VmReimage(id, imageIDInt)
	})
}

func (ln *LunaNode) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		instanceIDInt, err := strconv.Atoi(imageTemplate.SourceInstance)
		if err != nil {
			return nil, fmt.Errorf("invalid instance ID: %s", imageTemplate.SourceInstance)
		}
		imageID, err := ln.api.VmSnapshot(instanceIDInt)
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             strconv.Itoa(imageID),
				SourceInstance: imageTemplate.SourceInstance,
			}, nil
		}
	} else if imageTemplate.SourceURL != "" {
		if imageTemplate.Format == "" {
			return nil, errors.New("missing image format (iso or qcow2)")
		}

		isVirtio := imageTemplate.Details != nil && imageTemplate.Details["disk_bus"] == "virtio"
		region := DEFAULT_REGION
		if len(imageTemplate.Regions) == 1 {
			region = imageTemplate.Regions[0]
		} else if len(imageTemplate.Regions) > 1 {
			return nil, errors.New("image can only be fetched to one region")
		}

		imageID, err := ln.api.ImageFetch(region, imageTemplate.SourceURL, imageTemplate.Format, isVirtio)
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:        strconv.Itoa(imageID),
				Format:    imageTemplate.Format,
				SourceURL: imageTemplate.SourceURL,
			}, nil
		}
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (ln *LunaNode) mapImage(apiImage *lnapi.Image) *compute.Image {
	image := &compute.Image{
		ID:      apiImage.ID,
		Name:    apiImage.Name,
		Regions: []string{apiImage.Region},
		Public:  strings.Contains(apiImage.Name, " (template)") || strings.Contains(apiImage.Name, " (ISO)"),
	}

	image.Size, _ = strconv.ParseInt(apiImage.Size, 10, 64)

	if apiImage.Status == "active" {
		image.Status = compute.ImageAvailable
	} else if apiImage.Status == "error" || apiImage.Status == "killed" {
		image.Status = "error"
	} else {
		image.Status = compute.ImagePending
	}

	return image
}

func (ln *LunaNode) FindImage(image *compute.Image) (string, error) {
	var searchTerms []string
	if image.Distribution != "" {
		searchTerms = append(searchTerms, image.Distribution)

		if image.Version != "" {
			searchTerms = append(searchTerms, image.Version)
		}
	}
	if image.Architecture != "" {
		if image.Architecture == compute.ArchAMD64 {
			searchTerms = append(searchTerms, "64-bit")
		} else if image.Architecture == compute.Archi386 {
			searchTerms = append(searchTerms, "32-bit")
		} else {
			searchTerms = append(searchTerms, string(image.Architecture))
		}
	}
	if image.Type == compute.TemplateImage {
		searchTerms = append(searchTerms, "(template)")
	} else if image.Type == compute.ISOImage {
		searchTerms = append(searchTerms, "(ISO)")
	}

	if len(searchTerms) == 0 {
		searchTerms = []string{"ubuntu", "64-bit", "(template)"}
	}

	apiImages, err := ln.ListImages()
	if err != nil {
		return "", err
	}

	var bestImage *compute.Image
	var bestID int // get highest ID, which corresponds to latest image satisfying specification

	for _, apiImage := range apiImages {
		if !apiImage.Public {
			continue
		}

		// check for search terms
		fail := false
		for _, term := range searchTerms {
			if !strings.Contains(strings.ToLower(apiImage.Name), term) {
				fail = true
				break
			}
		}
		if fail {
			continue
		}

		// verify region requirements are satisfied
		if len(image.Regions) > 0 && len(apiImage.Regions) > 0 && !utils.IsSliceSubset(apiImage.Regions, image.Regions) {
			continue
		}

		imageID, _ := strconv.Atoi(image.ID)

		if bestImage == nil || imageID > bestID {
			bestImage = apiImage
			bestID = imageID
		}
	}

	if bestImage == nil {
		return "", nil
	} else {
		return bestImage.ID, nil
	}
}

func (ln *LunaNode) ListImages() ([]*compute.Image, error) {
	apiImages, err := ln.api.ImageList("")
	if err != nil {
		return nil, err
	}
	images := make([]*compute.Image, len(apiImages))
	for i, apiImage := range apiImages {
		images[i] = ln.mapImage(apiImage)
	}
	return images, nil
}

func (ln *LunaNode) GetImage(imageID string) (*compute.Image, error) {
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, fmt.Errorf("invalid image ID: %s", imageID)
	}
	apiImage, err := ln.api.ImageDetails(imageIDInt)
	if err != nil {
		return nil, err
	} else {
		return ln.mapImage(apiImage), nil
	}
}

func (ln *LunaNode) DeleteImage(imageID string) error {
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return fmt.Errorf("invalid image ID: %s", imageID)
	}
	return ln.api.ImageDelete(imageIDInt)
}

func (ln *LunaNode) ListFlavors() ([]*compute.Flavor, error) {
	apiPlans, err := ln.api.PlanList()
	if err != nil {
		return nil, err
	}
	flavors := make([]*compute.Flavor, len(apiPlans))
	for i, plan := range apiPlans {
		flavors[i] = &compute.Flavor{
			ID:   plan.ID,
			Name: plan.Name,
		}
		flavors[i].MemoryMB, _ = strconv.Atoi(plan.RAM)
		flavors[i].NumCores, _ = strconv.Atoi(plan.Vcpu)
		flavors[i].DiskGB, _ = strconv.Atoi(plan.Storage)
		flavors[i].TransferGB, _ = strconv.Atoi(plan.Bandwidth)
	}
	return flavors, nil
}

func (ln *LunaNode) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := ln.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
