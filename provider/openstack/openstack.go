package openstack

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "github.com/LunaNode/gophercloud"
import "github.com/LunaNode/gophercloud/openstack"
import "github.com/LunaNode/gophercloud/pagination"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/flavors"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/servers"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/startstop"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/floatingip"
import "github.com/LunaNode/gophercloud/openstack/image/v1/image"

import "errors"
import "fmt"
import "strconv"
import "strings"
import "time"

const DEFAULT_NAME = "cloug"

type OpenStack struct {
	ComputeClient *gophercloud.ServiceClient
	ImageClient   *gophercloud.ServiceClient
}

func MakeOpenStack(identityEndpoint string, username string, password string, tenantName string) *OpenStack {
	os := new(OpenStack)
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: identityEndpoint,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		panic(err)
	}
	os.ComputeClient, err = openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		panic(err)
	}
	os.ImageClient, err = openstack.NewImageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		panic(err)
	}
	return os
}

func (os *OpenStack) ComputeService() compute.Service {
	return os
}

func (os *OpenStack) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "ACTIVE" {
		return compute.StatusOnline
	} else if status == "SHUTOFF" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (os *OpenStack) serverToInstance(server *servers.Server) *compute.Instance {
	instance := &compute.Instance{
		ID:     server.ID,
		Name:   server.Name,
		Status: os.mapInstanceStatus(server.Status),
		IP:     server.AccessIPv4,
	}

	servers.ListAddresses(os.ComputeClient, server.ID).EachPage(func(page pagination.Page) (bool, error) {
		addresses, err := servers.ExtractAddresses(page)
		if err != nil {
			return false, err
		}

		for _, networkAddresses := range addresses {
			for _, addr := range networkAddresses {
				if utils.IsPrivate(addr.Address) {
					if instance.PrivateIP == "" || (utils.GetIPVersion(addr.Address) == 4 && utils.GetIPVersion(instance.PrivateIP) != 4) {
						instance.PrivateIP = addr.Address
					}
				} else if instance.IP == "" || (utils.GetIPVersion(addr.Address) == 4 && utils.GetIPVersion(instance.IP) != 4) {
					instance.IP = addr.Address
				}
			}
		}
		return true, nil
	})

	return instance
}

func (os *OpenStack) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(os, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(os, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	password := instance.Password
	if password == "" {
		password = utils.Uid(16)
	}

	opts := servers.CreateOpts{
		Name:             instance.Name,
		ImageRef:         imageID,
		FlavorRef:        flavorID,
		AdminPass:        password,
		UserData:         []byte("#cloud-config\npassword: " + password + "\nchpasswd: { expire: False }\nssh_pwauth: True\n"),
		AvailabilityZone: instance.Region,
	}

	if instance.NetworkID != "" {
		opts.Networks = []servers.Network{{UUID: instance.NetworkID}}
	}

	if opts.Name == "" {
		opts.Name = DEFAULT_NAME
	}

	createResult := servers.Create(os.ComputeClient, opts)
	server, err := createResult.Extract()
	if err != nil {
		return nil, err
	}

	// try to associate floating IP with this VM
	// do asynchronously since it might fail until network port is created
	go func() {
		for try := 0; try < 6; try++ {
			time.Sleep(4 * time.Second)

			// find a free floating IP, or find the IP matching requested IP
			var freeFloatingIP *floatingip.FloatingIP
			err := floatingip.List(os.ComputeClient).EachPage(func(page pagination.Page) (bool, error) {
				floatingIPs, err := floatingip.ExtractFloatingIPs(page)
				if err != nil {
					return false, err
				}

				for _, floatingIP := range floatingIPs {
					if floatingIP.InstanceID == "" && (instance.IP == "" || floatingIP.IP == instance.IP) {
						freeFloatingIP = &floatingIP
						return false, nil
					}
				}
				return true, nil
			})
			if err != nil {
				continue
			} else if freeFloatingIP == nil {
				continue
			}

			// associate it
			err = floatingip.Associate(os.ComputeClient, server.ID, freeFloatingIP.IP).ExtractErr()
			if err == nil {
				break
			}
		}
	}()

	return &compute.Instance{
		ID:       server.ID,
		Name:     server.Name,
		Status:   os.mapInstanceStatus(server.Status),
		Password: password,
	}, nil
}

func (os *OpenStack) DeleteInstance(instanceID string) error {
	return servers.Delete(os.ComputeClient, instanceID).ExtractErr()
}

func (os *OpenStack) ListInstances() ([]*compute.Instance, error) {
	var instances []*compute.Instance

	err := servers.List(os.ComputeClient, servers.ListOpts{}).EachPage(func(page pagination.Page) (bool, error) {
		list, err := servers.ExtractServers(page)
		if err != nil {
			return false, err
		}

		for _, server := range list {
			instances = append(instances, os.serverToInstance(&server))
		}
		return true, nil
	})

	if err != nil {
		return nil, err
	} else {
		return instances, nil
	}
}

func (os *OpenStack) GetInstance(instanceID string) (*compute.Instance, error) {
	server, err := servers.Get(os.ComputeClient, instanceID).Extract()
	if err != nil {
		return nil, err
	} else {
		return os.serverToInstance(server), nil
	}
}

func (os *OpenStack) StartInstance(instanceID string) error {
	return startstop.Start(os.ComputeClient, instanceID).ExtractErr()
}

func (os *OpenStack) StopInstance(instanceID string) error {
	return startstop.Stop(os.ComputeClient, instanceID).ExtractErr()
}

func (os *OpenStack) RebootInstance(instanceID string) error {
	return servers.Reboot(os.ComputeClient, instanceID, servers.HardReboot).ExtractErr()
}

func (os *OpenStack) GetVNC(instanceID string) (string, error) {
	return servers.Vnc(os.ComputeClient, instanceID, servers.NoVnc).Extract()
}

func (os *OpenStack) RenameInstance(instanceID string, name string) error {
	opts := servers.UpdateOpts{
		Name: name,
	}
	_, err := servers.Update(os.ComputeClient, instanceID, opts).Extract()
	return err
}

func (os *OpenStack) ReimageInstance(instanceID string, image *compute.Image) error {
	imageID, err := common.GetMatchingImageID(os, image)
	if err != nil {
		return err
	}
	opts := servers.RebuildOpts{
		ImageID: imageID,
	}
	_, err = servers.Rebuild(os.ComputeClient, instanceID, opts).Extract()
	return err
}

func (os *OpenStack) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		opts := servers.CreateImageOpts{
			Name: imageTemplate.Name,
		}
		imageID, err := servers.CreateImage(os.ComputeClient, imageTemplate.SourceInstance, opts).ExtractImageID()
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             imageID,
				Name:           imageTemplate.Name,
				SourceInstance: imageTemplate.SourceInstance,
			}, nil
		}
	} else if imageTemplate.SourceURL != "" {
		if imageTemplate.Format == "" {
			return nil, errors.New("missing image format (e.g. iso, qcow2, raw)")
		}
		opts := image.CreateOpts{
			Name:            imageTemplate.Name,
			ContainerFormat: "bare",
			DiskFormat:      imageTemplate.Format,
			CopyFrom:        imageTemplate.SourceURL,
		}
		createResult := image.Create(os.ImageClient, opts)
		apiImage, err := createResult.Extract()
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:     apiImage.ID,
				Name:   apiImage.Name,
				Format: apiImage.DiskFormat,
			}, nil
		}
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (os *OpenStack) mapImage(apiImage *image.Image) *compute.Image {
	image := &compute.Image{
		ID:     apiImage.ID,
		Name:   apiImage.Name,
		Public: apiImage.Visibility == "public",
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

func (os *OpenStack) FindImage(image *compute.Image) (string, error) {
	return "", errors.New("operation not supported")
}

func (os *OpenStack) ListImages() ([]*compute.Image, error) {
	return nil, errors.New("operation not supported")
}

func (os *OpenStack) GetImage(imageID string) (*compute.Image, error) {
	apiImage, err := image.Get(os.ImageClient, imageID).Extract()
	if err != nil {
		return nil, err
	} else {
		return os.mapImage(apiImage), nil
	}
}

func (os *OpenStack) DeleteImage(imageID string) error {
	err := image.Delete(os.ImageClient, imageID).ExtractErr()
	if err != nil && !strings.Contains(err.Error(), "Image with identifier "+imageID+" not found") {
		return err
	} else {
		return nil
	}
}

func (os *OpenStack) ListFlavors() ([]*compute.Flavor, error) {
	var flavorList []*compute.Flavor
	flavorPager := flavors.ListDetail(os.ComputeClient, flavors.ListOpts{})
	err := flavorPager.EachPage(func(page pagination.Page) (bool, error) {
		pageFlavors, err := flavors.ExtractFlavors(page)
		if err != nil {
			return false, err
		}

		for _, flavor := range pageFlavors {
			flavorList = append(flavorList, &compute.Flavor{
				ID:       flavor.ID,
				Name:     flavor.Name,
				MemoryMB: flavor.RAM,
				NumCores: flavor.VCPUs,
				DiskGB:   flavor.Disk,
			})
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	} else {
		return flavorList, nil
	}
}

func (os *OpenStack) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := os.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}
