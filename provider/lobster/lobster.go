package lobster

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"

import "github.com/LunaNode/lobster/api"

import "errors"
import "fmt"
import "strconv"
import "strings"

const DEFAULT_NAME = "cloug"

type Lobster struct {
	client *api.Client
}

func MakeLobster(url string, apiId string, apiKey string) *Lobster {
	lobster := new(Lobster)
	lobster.client = &api.Client{
		Url:    url,
		ApiId:  apiId,
		ApiKey: apiKey,
	}
	return lobster
}

func (lobster *Lobster) ComputeService() compute.Service {
	return lobster
}

func (lobster *Lobster) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "Online" {
		return compute.StatusOnline
	} else if status == "Offline" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (lobster *Lobster) vmToInstance(vm *api.VirtualMachine, details *api.VirtualMachineDetails) *compute.Instance {
	instance := &compute.Instance{
		ID:        strconv.Itoa(vm.Id),
		Name:      vm.Name,
		Region:    vm.Region,
		IP:        vm.ExternalIP,
		PrivateIP: vm.PrivateIP,
		Flavor:    compute.Flavor{ID: strconv.Itoa(vm.PlanId)},
		Details:   make(map[string]string),
	}

	if details != nil {
		instance.Status = lobster.mapInstanceStatus(details.Status)
		instance.BandwidthUsed = details.BandwidthUsed

		if details.LoginDetails != "" {
			// parse login details if it is one of:
			//  "username: ABC; password: XYZ"
			//  "password: XYZ"
			// if this fails, then just set it in the details map
			for _, loginPart := range strings.Split(details.LoginDetails, "; ") {
				parts := strings.Split(loginPart, ": ")
				if len(parts) == 2 {
					if parts[0] == "username" {
						instance.Username = parts[1]
					} else if parts[0] == "password" {
						instance.Password = parts[1]
					}
				}
			}

			if instance.Username == "" && instance.Password == "" {
				instance.Details["Login Details"] = details.LoginDetails
			}
		}
	}

	return instance
}

func (this *Lobster) findMatchingPlan(ram int, storage int, cpu int) (*api.Plan, error) {
	plans, err := this.client.PlanList()
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		if plan.Ram == ram && plan.Storage == storage && plan.Cpu == cpu {
			return plan, nil
		}
	}
	return nil, nil
}

func (lobster *Lobster) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(lobster, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(lobster, &instance.Flavor)
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

	name := instance.Name
	if name == "" {
		name = DEFAULT_NAME
	}

	vmID, err := lobster.client.VmCreate(name, flavorIDInt, imageIDInt)
	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID: strconv.Itoa(vmID),
		}, nil
	}
}

func (lobster *Lobster) instanceAction(instanceID string, f func(id int) error) error {
	instanceIDInt, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %s", instanceID)
	} else {
		return f(instanceIDInt)
	}
}

func (lobster *Lobster) DeleteInstance(instanceID string) error {
	return lobster.instanceAction(instanceID, lobster.client.VmDelete)
}

func (lobster *Lobster) ListInstances() ([]*compute.Instance, error) {
	vms, err := lobster.client.VmList()
	if err != nil {
		return nil, err
	}
	instances := make([]*compute.Instance, len(vms))
	for i, vm := range vms {
		instances[i] = lobster.vmToInstance(vm, nil)
	}
	return instances, nil
}

func (lobster *Lobster) GetInstance(instanceID string) (*compute.Instance, error) {
	instanceIDInt, _ := strconv.Atoi(instanceID)
	response, err := lobster.client.VmInfo(instanceIDInt)
	if err != nil {
		return nil, err
	} else {
		return lobster.vmToInstance(response.VirtualMachine, response.Details), nil
	}
}

func (lobster *Lobster) StartInstance(instanceID string) error {
	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAction(id, "start", "")
	})
}

func (lobster *Lobster) StopInstance(instanceID string) error {
	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAction(id, "stop", "")
	})
}

func (lobster *Lobster) RebootInstance(instanceID string) error {
	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAction(id, "reboot", "")
	})
}

func (lobster *Lobster) GetVNC(instanceID string) (string, error) {
	var url string
	err := lobster.instanceAction(instanceID, func(id int) error {
		var err error
		url, err = lobster.client.VmVnc(id)
		return err
	})
	return url, err
}

func (lobster *Lobster) RenameInstance(instanceID string, name string) error {
	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAction(id, "rename", name)
	})
}

func (lobster *Lobster) ReimageInstance(instanceID string, image *compute.Image) error {
	imageID, err := common.GetMatchingImageID(lobster, image)
	if err != nil {
		return err
	}
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return fmt.Errorf("invalid image ID: %s", imageID)
	}

	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmReimage(id, imageIDInt)
	})
}

func (lobster *Lobster) ResizeInstance(instanceID string, flavor *compute.Flavor) error {
	flavorID, err := common.GetMatchingFlavorID(lobster, flavor)
	if err != nil {
		return err
	}
	flavorIDInt, err := strconv.Atoi(flavorID)
	if err != nil {
		return fmt.Errorf("invalid flavor ID: %s", flavorID)
	}

	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmResize(id, flavorIDInt)
	})
}

func (lobster *Lobster) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		instanceIDInt, err := strconv.Atoi(imageTemplate.SourceInstance)
		if err != nil {
			return nil, fmt.Errorf("invalid instance ID: %s", imageTemplate.SourceInstance)
		}

		name := DEFAULT_NAME
		if imageTemplate.Name != "" {
			name = imageTemplate.Name
		}

		imageID, err := lobster.client.VmSnapshot(instanceIDInt, name)
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
			return nil, errors.New("missing image format")
		} else if len(imageTemplate.Regions) != 1 {
			return nil, errors.New("image must specify exactly one region")
		}

		name := DEFAULT_NAME
		if imageTemplate.Name != "" {
			name = imageTemplate.Name
		}

		imageID, err := lobster.client.ImageFetch(imageTemplate.Regions[0], name, imageTemplate.SourceURL, imageTemplate.Format)
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:        strconv.Itoa(imageID),
				Name:      name,
				Regions:   imageTemplate.Regions,
				Format:    imageTemplate.Format,
				SourceURL: imageTemplate.SourceURL,
			}, nil
		}
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (lobster *Lobster) mapImage(apiImage *api.Image, details *api.ImageDetails) *compute.Image {
	image := &compute.Image{
		ID:      strconv.Itoa(apiImage.Id),
		Name:    apiImage.Name,
		Regions: []string{apiImage.Region},
	}

	if apiImage.Status == "active" {
		image.Status = compute.ImageAvailable
	} else if apiImage.Status == "pending" {
		image.Status = compute.ImagePending
	} else {
		if details != nil {
			image.Status = compute.ImageStatus(details.Status)
		} else {
			image.Status = compute.ImageStatus(apiImage.Status)
		}
	}

	if details != nil {
		image.Size = details.Size
	}

	return image
}

func (lobster *Lobster) FindImage(image *compute.Image) (string, error) {
	return "", fmt.Errorf("operation not supported")
}

func (lobster *Lobster) ListImages() ([]*compute.Image, error) {
	apiImages, err := lobster.client.ImageList()
	if err != nil {
		return nil, err
	}
	images := make([]*compute.Image, len(apiImages))
	for i, apiImage := range apiImages {
		images[i] = lobster.mapImage(apiImage, nil)
	}
	return images, nil
}

func (lobster *Lobster) GetImage(imageID string) (*compute.Image, error) {
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, fmt.Errorf("invalid image ID: %s", imageID)
	}
	response, err := lobster.client.ImageInfo(imageIDInt)
	if err != nil {
		return nil, err
	} else {
		return lobster.mapImage(response.Image, response.Details), nil
	}
}

func (lobster *Lobster) DeleteImage(imageID string) error {
	imageIDInt, err := strconv.Atoi(imageID)
	if err != nil {
		return fmt.Errorf("invalid image ID: %s", imageID)
	}
	return lobster.client.ImageDelete(imageIDInt)
}

func (lobster *Lobster) ListInstanceAddresses(instanceID string) ([]*compute.Address, error) {
	var addresses []*compute.Address
	err := lobster.instanceAction(instanceID, func(id int) error {
		apiAddresses, err := lobster.client.VmAddresses(id)
		for _, addr := range apiAddresses {
			addresses = append(addresses, &compute.Address{
				ID:        instanceID + ":" + addr.Ip,
				IP:        addr.Ip,
				PrivateIP: addr.PrivateIp,
				CanDNS:    addr.CanRdns,
				Hostname:  addr.Hostname,
			})
		}
		return err
	})
	return addresses, err
}

func (lobster *Lobster) AddAddressToInstance(instanceID string, address *compute.Address) error {
	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAddressAdd(id)
	})
}

func (lobster *Lobster) RemoveAddressFromInstance(instanceID string, addressID string) error {
	addrs, err := lobster.ListInstanceAddresses(instanceID)
	if err != nil {
		return fmt.Errorf("failed to list instance addresses: %v", err)
	}
	var addr *compute.Address
	for _, a := range addrs {
		if a.ID == addressID {
			addr = a
			break
		}
	}
	if addr == nil {
		return fmt.Errorf("instance does not have the specified address")
	}

	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAddressRemove(id, addr.IP, addr.PrivateIP)
	})
}

func (lobster *Lobster) SetAddressHostname(addressID string, hostname string) error {
	parts := strings.Split(addressID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("address ID must contain two colon-separated parts")
	}
	instanceID := parts[0]
	ip := parts[1]

	return lobster.instanceAction(instanceID, func(id int) error {
		return lobster.client.VmAddressRdns(id, ip, hostname)
	})
}

func (lobster *Lobster) ListFlavors() ([]*compute.Flavor, error) {
	apiPlans, err := lobster.client.PlanList()
	if err != nil {
		return nil, err
	}
	flavors := make([]*compute.Flavor, len(apiPlans))
	for i, plan := range apiPlans {
		flavors[i] = &compute.Flavor{
			ID:         strconv.Itoa(plan.Id),
			Name:       plan.Name,
			MemoryMB:   plan.Ram,
			NumCores:   plan.Cpu,
			DiskGB:     plan.Storage,
			TransferGB: plan.Bandwidth,
		}
	}
	return flavors, nil
}

func (lobster *Lobster) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := lobster.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
