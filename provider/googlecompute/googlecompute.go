package googlecompute

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "golang.org/x/oauth2"
import "golang.org/x/oauth2/google"
import "golang.org/x/oauth2/jwt"
import gcompute "google.golang.org/api/compute/v1"
import "google.golang.org/api/googleapi"

import "fmt"
import "strings"
import "time"

const DEFAULT_REGION = "us-central1-f"

type OperationCall interface {
	Do(opts ...googleapi.CallOption) (*gcompute.Operation, error)
}

type GoogleCompute struct {
	project string
	service *gcompute.Service
}

func MakeGoogleCompute(email string, privateKey string, project string) (*GoogleCompute, error) {
	conf := &jwt.Config{
		Email:      email,
		PrivateKey: []byte(privateKey),
		Scopes:     []string{"https://www.googleapis.com/auth/compute"},
		TokenURL:   google.JWTTokenURL,
	}
	client := conf.Client(oauth2.NoContext)
	service, err := gcompute.New(client)
	if err != nil {
		return nil, err
	}
	return &GoogleCompute{
		service: service,
		project: project,
	}, nil
}

func (gc *GoogleCompute) ComputeService() compute.Service {
	return gc
}

func (gc *GoogleCompute) waitForOperation(call OperationCall) (*gcompute.Operation, error) {
	operation, err := call.Do()
	if err != nil {
		return nil, err
	}
	for range time.Tick(time.Second) {
		operation, err = gc.service.ZoneOperations.Get(gc.project, basename(operation.Zone), operation.Name).Do()
		if err != nil {
			return nil, fmt.Errorf("error getting update on operation: %v", err)
		}

		if operation.Status == "DONE" {
			break
		}
	}
	if operation.Error != nil {
		opErr := operation.Error.Errors[0]
		return nil, fmt.Errorf("%s: %s", opErr.Code, opErr.Message)
	} else {
		return operation, nil
	}
}

func (gc *GoogleCompute) mapInstanceStatus(state string) compute.InstanceStatus {
	if state == "RUNNING" {
		return compute.StatusOnline
	} else if state == "STOPPED" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(state))
	}
}

func (gc *GoogleCompute) mapInstance(apiInstance *gcompute.Instance) *compute.Instance {
	instance := &compute.Instance{
		ID:     fmt.Sprintf("%s:%s", basename(apiInstance.Zone), apiInstance.Name),
		Name:   apiInstance.Name,
		Region: basename(apiInstance.Zone),
		Status: gc.mapInstanceStatus(apiInstance.Status),
	}

	for _, nic := range apiInstance.NetworkInterfaces {
		if utils.IsPrivate(nic.NetworkIP) {
			instance.PrivateIP = nic.NetworkIP
		} else {
			instance.IP = nic.NetworkIP
		}
	}

	return instance
}

func (gc *GoogleCompute) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(gc, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(gc, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	name := utils.UidAlphabet(24, []rune("abcdefghijklmnopqrstuvwxyz"))

	region := instance.Region
	if region == "" {
		region = DEFAULT_REGION
	}

	apiInstance := gcompute.Instance{
		Name:        name,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", region, flavorID),
		Disks: []*gcompute.AttachedDisk{
			&gcompute.AttachedDisk{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &gcompute.AttachedDiskInitializeParams{
					SourceImage: imageID,
					DiskSizeGb:  int64(instance.Flavor.DiskGB),
				},
			},
		},
		NetworkInterfaces: []*gcompute.NetworkInterface{
			&gcompute.NetworkInterface{
				Network: "global/networks/default",
				AccessConfigs: []*gcompute.AccessConfig{
					&gcompute.AccessConfig{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},
	}

	password := instance.Password
	if password == "" {
		password = utils.Uid(16)
	}
	userData := "#cloud-config\npassword: " + password + "\nchpasswd: { expire: False }\nssh_pwauth: True\n"
	apiInstance.Metadata = &gcompute.Metadata{
		Items: []*gcompute.MetadataItems{
			&gcompute.MetadataItems{
				Key:   "user-data",
				Value: &userData,
			},
		},
	}

	operation, err := gc.waitForOperation(gc.service.Instances.Insert(gc.project, region, &apiInstance))
	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID:       fmt.Sprintf("%s:%s", basename(operation.Zone), name),
			Password: password,
		}, nil
	}
}

func (gc *GoogleCompute) instanceAction(instanceID string, f func(string, string) error) error {
	parts := strings.Split(instanceID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("instance ID does not contain two colon-separated parts")
	}
	zone := parts[0]
	name := parts[1]
	return f(zone, name)
}

func (gc *GoogleCompute) DeleteInstance(instanceID string) error {
	return gc.instanceAction(instanceID, func(zone string, name string) error {
		_, err := gc.waitForOperation(gc.service.Instances.Delete(gc.project, zone, name))
		return err
	})
}

func (gc *GoogleCompute) ListInstances() ([]*compute.Instance, error) {
	return nil, fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) GetInstance(instanceID string) (*compute.Instance, error) {
	var instance *compute.Instance
	err := gc.instanceAction(instanceID, func(zone string, name string) error {
		apiInstance, err := gc.service.Instances.Get(gc.project, zone, name).Do()
		if err != nil {
			return err
		}
		instance = gc.mapInstance(apiInstance)
		return nil
	})
	return instance, err
}

func (gc *GoogleCompute) StartInstance(instanceID string) error {
	return gc.instanceAction(instanceID, func(zone string, name string) error {
		_, err := gc.waitForOperation(gc.service.Instances.Start(gc.project, zone, name))
		return err
	})
}

func (gc *GoogleCompute) StopInstance(instanceID string) error {
	return gc.instanceAction(instanceID, func(zone string, name string) error {
		_, err := gc.waitForOperation(gc.service.Instances.Stop(gc.project, zone, name))
		return err
	})
}

func (gc *GoogleCompute) RebootInstance(instanceID string) error {
	return gc.instanceAction(instanceID, func(zone string, name string) error {
		_, err := gc.waitForOperation(gc.service.Instances.Reset(gc.project, zone, name))
		return err
	})
}

func (gc *GoogleCompute) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	return nil, fmt.Errorf("not implemented")
}

func (gc *GoogleCompute) mapImage(apiImage *gcompute.Image, region string) *compute.Image {
	image := &compute.Image{
		ID:   apiImage.SelfLink,
		Name: apiImage.Name,
		Size: apiImage.DiskSizeGb * 1024 * 1024 * 1024,
	}

	if apiImage.Status == "READY" {
		image.Status = compute.ImageAvailable
	} else if apiImage.Status == "PENDING" {
		image.Status = compute.ImagePending
	} else {
		image.Status = compute.ImageStatus(strings.ToLower(apiImage.Status))
	}

	return image
}

func (gc *GoogleCompute) FindImage(image *compute.Image) (string, error) {
	return "", fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) ListImages() ([]*compute.Image, error) {
	return nil, fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) GetImage(imageID string) (*compute.Image, error) {
	return nil, fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) DeleteImage(imageID string) error {
	return fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) ListFlavors() ([]*compute.Flavor, error) {
	return nil, fmt.Errorf("operation not supported")
}

func (gc *GoogleCompute) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := gc.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
