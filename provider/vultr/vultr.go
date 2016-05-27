package vultr

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"

import vultr "github.com/LunaNode/vultr/lib"

import "fmt"
import "strconv"
import "strings"

const DEFAULT_NAME = "cloug"
const DEFAULT_REGION = "New Jersey"

type Vultr struct {
	client *vultr.Client
}

func (vt *Vultr) ComputeService() compute.Service {
	return vt
}

func (vt *Vultr) findOSByName(name string) (int, error) {
	osList, err := vt.client.GetOS()
	if err != nil {
		return 0, err
	}

	for _, os := range osList {
		if os.Name == name {
			return os.ID, nil
		}
	}

	return 0, fmt.Errorf("no OS found matching %s", name)
}

func (vt *Vultr) findRegion(str string) (int, error) {
	// if str is decimal integer, then use that
	id, err := strconv.Atoi(str)
	if err == nil {
		return id, nil
	}

	regions, err := vt.client.GetRegions()
	if err != nil {
		return 0, fmt.Errorf("error listing regions: %v", err)
	}
	for _, region := range regions {
		if strings.ToLower(region.Name) == strings.ToLower(str) {
			return region.ID, nil
		}
	}
	return 0, fmt.Errorf("could not find region with name matching %s", str)
}

func (vt *Vultr) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(vt, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(vt, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	flavorIDInt, err := strconv.Atoi(flavorID)
	if err != nil {
		return nil, fmt.Errorf("invalid flavor ID: %s", flavorID)
	}

	name := instance.Name
	if name == "" {
		name = DEFAULT_NAME
	}

	serverOptions := &vultr.ServerOptions{
		PrivateNetworking: true,
		IPV6:              true,
	}

	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return nil, fmt.Errorf("malformed image ID: missing colon")
	}
	if imageParts[0] == "iso" {
		customOSID, err := vt.findOSByName("Custom")
		if err != nil {
			return nil, fmt.Errorf("failed to get custom OS for creation from ISO: %v", err)
		}
		serverOptions.OS = customOSID
		serverOptions.ISO, _ = strconv.Atoi(imageParts[1])
	} else if imageParts[0] == "os" {
		serverOptions.OS, _ = strconv.Atoi(imageParts[1])
	} else if imageParts[0] == "snapshot" {
		snapshotOSID, err := vt.findOSByName("Snapshot")
		if err != nil {
			return nil, fmt.Errorf("failed to get snapshot OS for creation from snapshot: %v", err)
		}
		serverOptions.OS = snapshotOSID
		serverOptions.Snapshot = imageParts[1]
	} else {
		return nil, fmt.Errorf("invalid image type " + imageParts[0])
	}

	region := DEFAULT_REGION
	if instance.Region != "" {
		region = instance.Region
	}
	regionID, err := vt.findRegion(region)
	if err != nil {
		return nil, fmt.Errorf("failed to identify region ID: %v", err)
	}

	server, err := vt.client.CreateServer(name, regionID, flavorIDInt, serverOptions)
	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID: server.ID,
		}, nil
	}
}

func (vt *Vultr) DeleteInstance(instanceID string) error {
	return vt.client.DeleteServer(instanceID)
}

func (vt *Vultr) mapInstanceStatus(status string, powerStatus string) compute.InstanceStatus {
	if status == "pending" {
		return "installing"
	} else if status == "active" {
		if powerStatus == "stopped" {
			return compute.StatusOffline
		} else if powerStatus == "running" {
			return compute.StatusOnline
		} else {
			return compute.InstanceStatus(powerStatus)
		}
	} else {
		return compute.InstanceStatus(status)
	}
}

func (vt *Vultr) serverToInstance(server vultr.Server) *compute.Instance {
	return &compute.Instance{
		ID:            server.ID,
		Name:          server.Name,
		Region:        fmt.Sprintf("%d", server.RegionID),
		IP:            server.MainIP,
		PrivateIP:     server.InternalIP,
		Flavor:        compute.Flavor{ID: strconv.Itoa(server.PlanID)},
		Password:      server.DefaultPassword,
		BandwidthUsed: int64(server.CurrentBandwidth) * 1024 * 1024 * 1024,
		Status:        vt.mapInstanceStatus(server.Status, server.PowerStatus),
	}
}

func (vt *Vultr) ListInstances() ([]*compute.Instance, error) {
	servers, err := vt.client.GetServers()
	if err != nil {
		return nil, err
	}
	instances := make([]*compute.Instance, len(servers))
	for i, server := range servers {
		instances[i] = vt.serverToInstance(server)
	}
	return instances, nil
}

func (vt *Vultr) GetInstance(instanceID string) (*compute.Instance, error) {
	server, err := vt.client.GetServer(instanceID)
	if err != nil {
		return nil, err
	} else {
		return vt.serverToInstance(server), nil
	}
}

func (vt *Vultr) StartInstance(instanceID string) error {
	return vt.client.StartServer(instanceID)
}

func (vt *Vultr) StopInstance(instanceID string) error {
	return vt.client.HaltServer(instanceID)
}

func (vt *Vultr) RebootInstance(instanceID string) error {
	return vt.client.RebootServer(instanceID)
}

func (vt *Vultr) GetVNC(instanceID string) (string, error) {
	server, err := vt.client.GetServer(instanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get server details: %v", err)
	} else if server.KVMUrl == "" {
		return "", fmt.Errorf("console is not ready yet")
	} else {
		return server.KVMUrl, nil
	}
}

func (vt *Vultr) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		name := DEFAULT_NAME
		if imageTemplate.Name != "" {
			name = imageTemplate.Name
		}

		snapshot, err := vt.client.CreateSnapshot(imageTemplate.SourceInstance, name)
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             fmt.Sprintf("snapshot:%s", snapshot.ID),
				SourceInstance: imageTemplate.SourceInstance,
			}, nil
		}
	} else if imageTemplate.SourceURL != "" {
		return nil, fmt.Errorf("creating image from source URL is not supported on vultr provider")
	} else {
		return nil, fmt.Errorf("neither source instance nor source URL is set")
	}
}

func (vt *Vultr) mapSnapshot(snapshot *vultr.Snapshot) *compute.Image {
	image := &compute.Image{
		ID:   fmt.Sprintf("snapshot:%s", snapshot.ID),
		Name: snapshot.Description,
		Size: snapshot.Size,
	}

	if snapshot.Status == "complete" {
		image.Status = compute.ImageAvailable
	} else {
		image.Status = compute.ImagePending
	}

	return image
}

func (vt *Vultr) FindImage(image *compute.Image) (string, error) {
	return "", fmt.Errorf("operation not supported")
}

func (vt *Vultr) ListImages() ([]*compute.Image, error) {
	var images []*compute.Image

	osList, err := vt.client.GetOS()
	if err != nil {
		return nil, err
	}
	for _, os := range osList {
		images = append(images, &compute.Image{
			ID:   fmt.Sprintf("os:%d", os.ID),
			Name: os.Name,
		})
	}

	snapshots, err := vt.client.GetSnapshots()
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		images = append(images, vt.mapSnapshot(&snapshot))
	}

	return images, nil
}

func (vt *Vultr) GetImage(imageID string) (*compute.Image, error) {
	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return nil, fmt.Errorf("malformed image identification: missing colon")
	} else if imageParts[0] != "snapshot" {
		return nil, fmt.Errorf("GetImage only supports snapshot images")
	}
	snapshots, err := vt.client.GetSnapshots()
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot.ID == imageParts[1] {
			return vt.mapSnapshot(&snapshot), nil
		}
	}
	return nil, fmt.Errorf("image not found")
}

func (vt *Vultr) DeleteImage(imageID string) error {
	imageParts := strings.SplitN(imageID, ":", 2)
	if len(imageParts) != 2 {
		return fmt.Errorf("malformed image identification: missing colon")
	} else if imageParts[0] != "snapshot" {
		return fmt.Errorf("can only delete snapshot images")
	}
	return vt.client.DeleteSnapshot(imageParts[1])
}

func (vt *Vultr) ListFlavors() ([]*compute.Flavor, error) {
	apiPlans, err := vt.client.GetPlans()
	if err != nil {
		return nil, err
	}
	flavors := make([]*compute.Flavor, len(apiPlans))
	for i, apiPlan := range apiPlans {
		flavors[i] = &compute.Flavor{
			ID:       strconv.Itoa(apiPlan.ID),
			Name:     apiPlan.Name,
			MemoryMB: apiPlan.RAM,
			NumCores: apiPlan.VCpus,
			DiskGB:   apiPlan.Disk,
		}
	}
	return flavors, nil
}

func (vt *Vultr) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := vt.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
