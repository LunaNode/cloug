package linode

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "github.com/LunaNode/go-linode"

import "errors"
import "fmt"
import "strconv"
import "strings"

const DEFAULT_DATACENTER = "newark"
const DEFAULT_STORAGE = 10

type Linode struct {
	client       *linode.Client
}

func MakeLinode(apiKey string) *Linode {
	return &Linode{
		client: linode.NewClient(apiKey),
	}
}

func (ln *Linode) ComputeService() compute.Service {
	return ln
}

func (ln *Linode) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "Running" {
		return compute.StatusOnline
	} else if status == "Powered Off" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (ln *Linode) linodeToInstance(linode *linode.Linode, ips []*linode.IP) *compute.Instance {
	instance := &compute.Instance{
		ID:        strconv.Itoa(linode.ID),
		Name:      linode.Label,
		Flavor:    compute.Flavor{
			ID: strconv.Itoa(linode.Plan),
			DiskGB: linode.TotalHD / 1024,
			MemoryMB: linode.TotalRAM,
		},
		Status: ln.mapInstanceStatus(linode.StatusString),
	}

	for _, ip := range ips {
		if ip.IsPublic == 1 {
			instance.IP = ip.Address
		} else {
			instance.PrivateIP = ip.Address
		}
	}

	return instance
}

func (ln *Linode) findKernel() (int, error) {
	kernels, err := ln.client.ListKernels()
	if err != nil {
		return 0, err
	}
	for _, kernel := range kernels {
		if strings.Contains(kernel.Label, "Latest 64 bit") {
			return kernel.ID, nil
		}
	}
	return 0, errors.New("no kernel found")
}

func (ln *Linode) findDatacenter(abbreviation string) (int, error) {
	datacenters, err := ln.client.ListDatacenters()
	if err != nil {
		return 0, err
	}
	for _, datacenter := range datacenters {
		if datacenter.Abbreviation == abbreviation {
			return datacenter.ID, nil
		}
	}
	return 0, errors.New("no matching datacenter found")
}

func (ln *Linode) findMatchingPlan(flavor *compute.Flavor) (*linode.Plan, error) {
	flavorID, err := common.GetMatchingFlavorID(ln, flavor)
	if err != nil {
		return nil, err
	}
	planID, err := strconv.Atoi(flavorID)
	if err != nil {
		return nil, fmt.Errorf("invalid flavor ID %s", flavorID)
	}
	plans, err := ln.client.ListPlans()
	if err != nil {
		return nil, fmt.Errorf("failed to list plans")
	}
	for _, plan := range plans {
		if plan.ID == planID {
			return plan, nil
		}
	}
	return nil, fmt.Errorf("could not find plan with id=%d", planID)
}

func (ln *Linode) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(ln, &instance.Image)
	if err != nil {
		return nil, err
	}
	plan, err := ln.findMatchingPlan(&instance.Flavor)
	if err != nil {
		return nil, err
	}
	kernelID, err := ln.findKernel()
	if err != nil {
		return nil, err
	}
	password := instance.Password
	if password == "" {
		password = utils.Uid(16)
	}

	datacenterAbbrev := instance.Region
	if datacenterAbbrev == "" {
		datacenterAbbrev = DEFAULT_DATACENTER
	}
	datacenterID, err := ln.findDatacenter(datacenterAbbrev)
	if err != nil {
		return nil, err
	}

	// create linode
	linodeID, err := ln.client.CreateLinode(datacenterID, plan.ID)
	if err != nil {
		return nil, err
	}

	// create disks
	storage := instance.Flavor.DiskGB
	if storage == 0 {
		storage = DEFAULT_STORAGE
	}

	totalDiskMB := storage * 1024
	swapSize := plan.RAM / 2
	diskSize := totalDiskMB - swapSize

	var diskID int
	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return nil, errors.New("malformed image ID: missing colon")
	}
	if imageParts[0] == "distribution" {
		distributionID, _ := strconv.Atoi(imageParts[1])
		diskID, _, err = ln.client.CreateDiskFromDistribution(linodeID, "cloug", distributionID, diskSize, password, "")
		if err != nil {
			ln.client.DeleteLinode(linodeID, false)
			return nil, err
		}
	} else if imageParts[0] == "image" {
		imageID, _ := strconv.Atoi(imageParts[1])
		diskID, _, err = ln.client.CreateDiskFromImage(linodeID, "cloug", imageID, diskSize, password, "")
		if err != nil {
			ln.client.DeleteLinode(linodeID, false)
			return nil, err
		}
	} else {
		return nil, errors.New("invalid image type " + imageParts[0])
	}

	swapID, _, err := ln.client.CreateDisk(linodeID, "cloug-swap", "swap", swapSize, linode.CreateDiskOptions{})
	if err != nil {
		ln.client.DeleteLinode(linodeID, false)
		return nil, err
	}

	_, err = ln.client.CreateConfig(linodeID, kernelID, "cloug", []int{diskID, swapID}, linode.CreateConfigOptions{})
	if err != nil {
		ln.client.DeleteLinode(linodeID, false)
		return nil, err
	} else {
		ln.client.BootLinode(linodeID)
		return &compute.Instance{
			ID: strconv.Itoa(linodeID),
			Password: password,
		}, nil
	}
}

func (ln *Linode) instanceAction(instanceID string, f func(id int) error) error {
	instanceIDInt, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %s", instanceID)
	} else {
		return f(instanceIDInt)
	}
}

func (ln *Linode) DeleteInstance(instanceID string) error {
	return ln.instanceAction(instanceID, func(id int) error {
		return ln.client.DeleteLinode(id, true)
	})
}

func (ln *Linode) ListInstances() ([]*compute.Instance, error) {
	linodes, err := ln.client.ListLinodes()
	if err != nil {
		return nil, err
	}
	instances := make([]*compute.Instance, len(linodes))
	for i, linode := range linodes {
		instances[i] = ln.linodeToInstance(linode, nil)
	}
	return instances, nil
}

func (ln *Linode) GetInstance(instanceID string) (*compute.Instance, error) {
	linodeID, _ := strconv.Atoi(instanceID)
	linode, err := ln.client.GetLinode(linodeID)
	if err != nil {
		return nil, err
	}
	ips, err := ln.client.ListIP(linodeID)
	if err != nil {
		return nil, err
	}
	return ln.linodeToInstance(linode, ips), nil
}

func (ln *Linode) StartInstance(instanceID string) error {
	return ln.instanceAction(instanceID, func(id int) error {
		_, err := ln.client.BootLinode(id)
		return err
	})
}

func (ln *Linode) StopInstance(instanceID string) error {
	return ln.instanceAction(instanceID, func(id int) error {
		_, err := ln.client.ShutdownLinode(id)
		return err
	})
}

func (ln *Linode) RebootInstance(instanceID string) error {
	return ln.instanceAction(instanceID, func(id int) error {
		_, err := ln.client.RebootLinode(id)
		return err
	})
}

func (ln *Linode) getDiskID(linodeID int) (int, error) {
	disks, err := ln.client.ListDisks(linodeID)
	if err != nil {
		return 0, err
	}
	for _, disk := range disks {
		if disk.Type != "swap" {
			return disk.ID, nil
		}
	}
	return 0, fmt.Errorf("no non-swap disks found on this Linode")
}

func (ln *Linode) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		linodeID, err := strconv.Atoi(imageTemplate.SourceInstance)
		if err != nil {
			return nil, fmt.Errorf("invalid instance ID: %s", imageTemplate.SourceInstance)
		}

		diskID, err := ln.getDiskID(linodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to find root disk: %v", err)
		}
		imageID, _, err := ln.client.ImagizeDisk(linodeID, diskID, "cloug image")
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             fmt.Sprintf("image:%d", imageID),
				SourceInstance: imageTemplate.SourceInstance,
			}, nil
		}
	} else if imageTemplate.SourceURL != "" {
		return nil, fmt.Errorf("creating image from source URL is not supported on linode provider")
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (ln *Linode) mapImage(apiImage *linode.Image) *compute.Image {
	image := &compute.Image{
		ID:      fmt.Sprintf("image:%d", apiImage.ID),
		Name:    apiImage.Label,
		Size:    int64(apiImage.MinSize) * 1024 * 1024,
	}

	if apiImage.Status == "available" {
		image.Status = compute.ImageAvailable
	} else {
		image.Status = compute.ImagePending
	}

	return image
}

func (ln *Linode) FindImage(image *compute.Image) (string, error) {
	return "", fmt.Errorf("operation not supported")
}

func (ln *Linode) ListImages() ([]*compute.Image, error) {
	var images []*compute.Image

	apiImages, err := ln.client.ListImages(false)
	if err != nil {
		return nil, err
	}
	for _, apiImage := range apiImages {
		images = append(images, ln.mapImage(apiImage))
	}

	distributions, err := ln.client.ListDistributions()
	if err != nil {
		return nil, err
	}
	for _, distribution := range distributions {
		images = append(images, &compute.Image{
			ID:   fmt.Sprintf("distribution:%d", distribution.ID),
			Name: distribution.Label,
		})
	}

	return images, nil
}

func (ln *Linode) GetImage(imageID string) (*compute.Image, error) {
	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return nil, fmt.Errorf("malformed image identification: missing colon")
	} else if imageParts[0] != "image" {
		return nil, fmt.Errorf("can only fetch info for images")
	}
	imageIDInt, err := strconv.Atoi(imageParts[1])
	image, err := ln.client.GetImage(imageIDInt)
	if err != nil {
		return nil, fmt.Errorf("invalid image ID: %s", imageID)
	}
	if err != nil {
		return nil, err
	} else {
		return ln.mapImage(image), nil
	}
}

func (ln *Linode) DeleteImage(imageID string) error {
	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return errors.New("malformed image identification: missing colon")
	} else if imageParts[0] != "image" {
		return errors.New("can only delete images")
	}
	imageIDInt, err := strconv.Atoi(imageParts[1])
	if err != nil {
		return fmt.Errorf("invalid image ID: %s", imageID)
	}
	return ln.client.DeleteImage(imageIDInt)
}

func (ln *Linode) ListFlavors() ([]*compute.Flavor, error) {
	apiPlans, err := ln.client.ListPlans()
	if err != nil {
		return nil, err
	}
	flavors := make([]*compute.Flavor, len(apiPlans))
	for i, plan := range apiPlans {
		flavors[i] = &compute.Flavor{
			ID:         strconv.Itoa(plan.ID),
			Name:       plan.Label,
			MemoryMB:   plan.RAM,
			NumCores:   plan.Cores,
			DiskGB:     plan.Disk,
			TransferGB: plan.Bandwidth,
		}
	}
	return flavors, nil
}

func (ln *Linode) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := ln.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
