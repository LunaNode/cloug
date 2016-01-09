package digitalocean

import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "github.com/digitalocean/godo"
import "golang.org/x/oauth2"

import "errors"
import "fmt"
import "strconv"
import "strings"
import "time"

const DEFAULT_NAME = "cloug"
const DEFAULT_REGION = "nyc3"

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

type DigitalOcean struct {
	client *godo.Client
}

func MakeDigitalOcean(token string) *DigitalOcean {
	do := new(DigitalOcean)
	tokenSource := &TokenSource{
		AccessToken: token,
	}
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	do.client = godo.NewClient(oauthClient)
	return do
}

func (do *DigitalOcean) ComputeService() compute.Service {
	return do
}

func (do *DigitalOcean) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "active" {
		return compute.StatusOnline
	} else if status == "off" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(status)
	}
}

func (do *DigitalOcean) dropletToInstance(droplet *godo.Droplet) *compute.Instance {
	instance := &compute.Instance{
		ID:     strconv.Itoa(droplet.ID),
		Name:   droplet.Name,
		Region: droplet.Region.Slug,
		Status: do.mapInstanceStatus(droplet.Status),
	}

	for _, addr4 := range droplet.Networks.V4 {
		if addr4.Type == "public" {
			instance.IP = addr4.IPAddress
		} else if addr4.Type == "private" {
			instance.PrivateIP = addr4.IPAddress
		}
	}

	return instance
}

func (do *DigitalOcean) processAction(dropletID int, actionID int) error {
	for i := 0; i < 10; i++ {
		action, _, err := do.client.DropletActions.Get(dropletID, actionID)
		if err != nil {
			return err
		} else if action.Status == "completed" {
			return nil
		} else if action.Status != "in-progress" {
			return errors.New("action status is " + action.Status)
		}
		time.Sleep(time.Second)
	}

	// still not done after 10 seconds?
	// for now, we assume it will eventually complete...
	return nil
}

func (do *DigitalOcean) imageID(image *compute.Image) (int, error) {
	var err error
	var imageIDStr string

	if image.ID == "" {
		imageIDStr, err = do.FindImage(image)
		if err != nil {
			return 0, fmt.Errorf("error finding image: %v", err)
		} else if imageIDStr == "" {
			return 0, fmt.Errorf("no matching image found")
		}
	} else {
		// we get the image because provided ID may not be direct identifier
		// e.g. snapshot will return snapshot:{snapshot name} ID
		image, err = do.GetImage(image.ID)
		if err != nil {
			return 0, err
		}
		imageIDStr = image.ID
	}

	imageID, err := strconv.Atoi(image.ID)
	if err != nil {
		panic(err)
	}

	return imageID, nil
}

func (do *DigitalOcean) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := do.imageID(&instance.Image)
	if err != nil {
		return nil, err
	}

	var flavorID string
	if instance.Flavor.ID == "" {
		flavorID, err = do.FindFlavor(&instance.Flavor)
		if err != nil {
			return nil, fmt.Errorf("error finding flavor: %v", err)
		} else if flavorID == "" {
			return nil, fmt.Errorf("no matching flavor found")
		}
	} else {
		flavorID = instance.Flavor.ID
	}

	password := instance.Password
	if password == "" {
		password = utils.Uid(16)
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   instance.Name,
		Region: instance.Region,
		Size:   flavorID,
		Image: godo.DropletCreateImage{
			ID: imageID,
		},
		IPv6:              true,
		PrivateNetworking: true,
		UserData:          fmt.Sprintf("#cloud-config\nchpasswd:\n list: |\n  root:%s\n expire: False\n", password),
	}

	if createRequest.Name == "" {
		createRequest.Name = DEFAULT_NAME
	}

	if createRequest.Region == "" {
		createRequest.Region = DEFAULT_REGION
	}

	droplet, _, err := do.client.Droplets.Create(createRequest)

	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID:       fmt.Sprintf("%d", droplet.ID),
			Name:     droplet.Name,
			Status:   do.mapInstanceStatus(droplet.Status),
			Username: "root",
			Password: password,
		}, nil
	}
}

func (do *DigitalOcean) DeleteInstance(instanceID string) error {
	dropletID, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID %s", instanceID)
	}
	_, err = do.client.Droplets.Delete(dropletID)
	return err
}

func (do *DigitalOcean) ListInstances() ([]*compute.Instance, error) {
	droplets, _, err := do.client.Droplets.List(&godo.ListOptions{PerPage: 500})
	if err != nil {
		return nil, err
	}

	instances := make([]*compute.Instance, len(droplets))
	for i, droplet := range droplets {
		instances[i] = do.dropletToInstance(&droplet)
	}
	return instances, nil
}

func (do *DigitalOcean) GetInstance(instanceID string) (*compute.Instance, error) {
	dropletID, err := strconv.Atoi(instanceID)
	if err != nil {
		return nil, fmt.Errorf("invalid instance ID %s", instanceID)
	}

	droplet, _, err := do.client.Droplets.Get(dropletID)
	if err != nil {
		return nil, fmt.Errorf("error getting droplet: %v", err)
	}

	instance := do.dropletToInstance(droplet)

	// list droplet actions
	var pendingActions []string
	actionList, _, err := do.client.Droplets.Actions(droplet.ID, &godo.ListOptions{PerPage: 500})
	if err == nil {
		for _, action := range actionList {
			if action.Status == "in-progress" {
				pendingActions = append(pendingActions, action.Type)
			}
		}
		if len(pendingActions) >= 1 {
			instance.Details = make(map[string]string)
			if len(pendingActions) == 1 {
				instance.Details["Pending action"] = pendingActions[0]
			} else {
				instance.Details["Pending actions"] = strings.Join(pendingActions, ", ")
			}
		}
	}

	return instance, nil
}

type DropletActionFunc func(id int) (*godo.Action, *godo.Response, error)

func (do *DigitalOcean) doAction(instanceID string, f DropletActionFunc) error {
	dropletID, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID %s", instanceID)
	}

	action, _, err := f(dropletID)
	if err != nil {
		return err
	} else {
		return do.processAction(dropletID, action.ID)
	}
}

func (do *DigitalOcean) StartInstance(instanceID string) error {
	return do.doAction(instanceID, do.client.DropletActions.PowerOn)
}

func (do *DigitalOcean) StopInstance(instanceID string) error {
	return do.doAction(instanceID, do.client.DropletActions.PowerOff)
}

func (do *DigitalOcean) RebootInstance(instanceID string) error {
	return do.doAction(instanceID, do.client.DropletActions.Reboot)
}

func (do *DigitalOcean) RenameInstance(instanceID string, name string) error {
	return do.doAction(instanceID, func(id int) (*godo.Action, *godo.Response, error) {
		return do.client.DropletActions.Rename(id, name)
	})
}

func (do *DigitalOcean) ReimageInstance(instanceID string, image *compute.Image) error {
	imageID, err := do.imageID(image)
	if err != nil {
		return err
	}

	return do.doAction(instanceID, func(id int) (*godo.Action, *godo.Response, error) {
		return do.client.DropletActions.RebuildByImageID(id, imageID)
	})
}

func (do *DigitalOcean) ResizeInstance(instanceID string, flavor *compute.Flavor) error {
	var err error
	var flavorID string

	if flavor.ID == "" {
		flavorID, err = do.FindFlavor(flavor)
		if err != nil {
			return fmt.Errorf("error finding flavor: %v", err)
		} else if flavorID == "" {
			return fmt.Errorf("no matching flavor found")
		}
	} else {
		flavorID = flavor.ID
	}

	return do.doAction(instanceID, func(id int) (*godo.Action, *godo.Response, error) {
		return do.client.DropletActions.Resize(id, flavorID, true)
	})
}

func (do *DigitalOcean) CreateImage(image *compute.Image) (*compute.Image, error) {
	if image.SourceInstance != "" {
		snapshotName := fmt.Sprintf("%s.%s", image.Name, utils.Uid(16))

		err := do.doAction(image.SourceInstance, func(id int) (*godo.Action, *godo.Response, error) {
			return do.client.DropletActions.Snapshot(id, snapshotName)
		})

		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             "snapshot:" + snapshotName,
				Name:           snapshotName,
				SourceInstance: image.SourceInstance,
			}, nil
		}
	} else if image.SourceURL != "" {
		return nil, errors.New("fetching image from URL is not supported")
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (do *DigitalOcean) mapImage(apiImage *godo.Image) *compute.Image {
	return &compute.Image{
		ID:      strconv.Itoa(apiImage.ID),
		Name:    fmt.Sprintf("%s %s", apiImage.Distribution, apiImage.Name),
		Regions: apiImage.Regions,
		Status:  compute.ImageAvailable,
		Public:  apiImage.Public,
		Size:    int64(apiImage.MinDiskSize) * 1024 * 1024 * 1024,
	}
}

func (do *DigitalOcean) FindImage(image *compute.Image) (string, error) {
	apiImages, _, err := do.client.Images.ListDistribution(&godo.ListOptions{PerPage: 500})
	if err != nil {
		return "", fmt.Errorf("error listing distribution images: %v", err)
	}

	matchDistribution := "ubuntu"
	if image.Distribution != "" {
		matchDistribution = image.Distribution
	}
	matchArchitecture := "x64"
	if image.Architecture == compute.Archi386 {
		matchArchitecture = "32"
	}

	var bestImage *godo.Image
	for _, apiImage := range apiImages {
		if !apiImage.Public {
			continue
		} else if strings.ToLower(apiImage.Distribution) != matchDistribution {
			continue
		} else if !strings.Contains(apiImage.Name, matchArchitecture) {
			continue
		}
		if bestImage == nil || apiImage.ID > bestImage.ID {
			bestImage = &apiImage
		}
	}

	if bestImage == nil {
		return "", nil
	} else {
		return strconv.Itoa(bestImage.ID), nil
	}
}

func (do *DigitalOcean) ListImages() ([]*compute.Image, error) {
	apiImages, _, err := do.client.Images.ListDistribution(&godo.ListOptions{PerPage: 500})
	if err != nil {
		return nil, err
	}
	images := make([]*compute.Image, len(apiImages))
	for i, apiImage := range apiImages {
		images[i] = do.mapImage(&apiImage)
	}
	return images, nil
}

func (do *DigitalOcean) GetImage(imageID string) (*compute.Image, error) {
	parts := strings.SplitN(imageID, ":", 2)
	if len(parts) == 2 {
		if parts[0] == "snapshot" {
			images, _, err := do.client.Images.ListUser(&godo.ListOptions{PerPage: 500})
			if err != nil {
				return nil, err
			}
			for _, image := range images {
				if image.Name == parts[1] {
					return do.mapImage(&image), nil
				}
			}
			// snapshot might be still pending
			return &compute.Image{
				Status: compute.ImagePending,
			}, nil
		} else {
			return nil, fmt.Errorf("invalid image prefix %s", parts[0])
		}
	} else {
		id, err := strconv.Atoi(imageID)
		if err != nil {
			return nil, fmt.Errorf("invalid image ID: %s", imageID)
		}
		image, _, err := do.client.Images.GetByID(id)
		if err != nil {
			return nil, err
		} else {
			return do.mapImage(image), nil
		}
	}
}

func (do *DigitalOcean) DeleteImage(imageIDStr string) error {
	imageID, err := do.imageID(&compute.Image{
		ID: imageIDStr,
	})
	if err != nil {
		return err
	}
	_, err = do.client.Images.Delete(imageID)
	return err
}

func (do *DigitalOcean) ListFlavors() ([]*compute.Flavor, error) {
	sizes, _, err := do.client.Sizes.List(&godo.ListOptions{PerPage: 500})
	if err != nil {
		return nil, err
	}
	flavors := make([]*compute.Flavor, len(sizes))
	for i, size := range sizes {
		flavors[i] = &compute.Flavor{
			ID:         size.Slug,
			Name:       size.Slug,
			Regions:    size.Regions,
			NumCores:   size.Vcpus,
			DiskGB:     size.Disk,
			MemoryMB:   size.Memory,
			TransferGB: int(size.Transfer * 1024),
		}
	}
	return flavors, nil
}

func (do *DigitalOcean) FindFlavor(flavor *compute.Flavor) (string, error) {
	if flavor.MemoryMB < 1024 {
		return "512mb", nil
	} else {
		return fmt.Sprintf("%dgb", flavor.MemoryMB/1024), nil
	}
}
