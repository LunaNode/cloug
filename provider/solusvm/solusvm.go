package solusvm

import "github.com/LunaNode/cloug/service/compute"

import "fmt"
import "strconv"
import "strings"

const DEFAULT_NAME = "cloug"
const DEFAULT_RAM = 512
const DEFAULT_DISK = 10
const DEFAULT_CORES = 1

type SolusVM struct {
	VirtType  string
	NodeGroup string
	Api       *API
}

func (solus *SolusVM) ComputeService() compute.Service {
	return solus
}

func (solus *SolusVM) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "online" {
		return compute.StatusOnline
	} else if status == "offline" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (solus *SolusVM) vmToInstance(id int, apiInfo *APIVmInfoResponse) *compute.Instance {
	instance := &compute.Instance{
		ID:        strconv.Itoa(id),
		IP:        apiInfo.Ip,
		PrivateIP: apiInfo.InternalIps,
		Status:    solus.mapInstanceStatus(apiInfo.State),
	}

	bwParts := strings.Split(apiInfo.Bandwidth, ",")
	if len(bwParts) >= 2 {
		instance.BandwidthUsed, _ = strconv.ParseInt(bwParts[1], 10, 64)
	}

	if solus.VirtType == "openvz" {
		instance.Actions = append(instance.Actions, compute.InstanceAction{
			Label:       "TUN/TAP",
			Description: "Enable or disable TUN/TAP.",
			Options: map[string]string{
				"enable":  "On",
				"disable": "Off",
			},
			Func: solus.vmTunTapFunc(id),
		})
	}

	return instance
}

func (solus *SolusVM) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	name := DEFAULT_NAME
	ram := DEFAULT_RAM
	disk := DEFAULT_DISK
	cores := DEFAULT_CORES

	if instance.Name != "" {
		name = instance.Name
	}
	if instance.Flavor.MemoryMB != 0 {
		ram = instance.Flavor.MemoryMB
	}
	if instance.Flavor.DiskGB != 0 {
		disk = instance.Flavor.DiskGB
	}
	if instance.Flavor.NumCores != 0 {
		cores = instance.Flavor.NumCores
	}

	vmID, password, err := solus.Api.VmCreate(solus.VirtType, solus.NodeGroup, name, instance.Image.ID, ram, disk, cores)
	if err != nil {
		return nil, err
	} else {
		return &compute.Instance{
			ID: strconv.Itoa(vmID),
			Password: password,
		}, nil
	}
}

func (solus *SolusVM) instanceAction(instanceID string, f func(id int) error) error {
	instanceIDInt, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %s", instanceID)
	} else {
		return f(instanceIDInt)
	}
}

func (solus *SolusVM) DeleteInstance(instanceID string) error {
	return solus.instanceAction(instanceID, solus.Api.VmDelete)
}

func (solus *SolusVM) ListInstances() ([]*compute.Instance, error) {
	return nil, fmt.Errorf("not implemented")
}

func (solus *SolusVM) GetInstance(instanceID string) (*compute.Instance, error) {
	instanceIDInt, _ := strconv.Atoi(instanceID)
	apiInfo, err := solus.Api.VmInfo(instanceIDInt)
	if err != nil {
		return nil, err
	} else {
		return solus.vmToInstance(instanceIDInt, apiInfo), nil
	}
}

func (solus *SolusVM) StartInstance(instanceID string) error {
	return solus.instanceAction(instanceID, solus.Api.VmStart)
}

func (solus *SolusVM) StopInstance(instanceID string) error {
	return solus.instanceAction(instanceID, solus.Api.VmStop)
}

func (solus *SolusVM) RebootInstance(instanceID string) error {
	return solus.instanceAction(instanceID, solus.Api.VmReboot)
}

func (solus *SolusVM) GetVNC(instanceID string) (string, error) {
	var url string
	err := solus.instanceAction(instanceID, func(id int) error {
		if solus.VirtType == "kvm" || solus.VirtType == "xen" {
			vncInfo, err := solus.Api.VmVnc(id)
			if err != nil {
				return err
			}
			url = fmt.Sprintf("vnc://%s:%s?password=%s", vncInfo.Ip, vncInfo.Port, vncInfo.Password)
		} else {
			consoleInfo, err := solus.Api.VmConsole(id)
			if err != nil {
				return err
			}
			url = fmt.Sprintf("ssh://%s:%s@%s:%s", consoleInfo.Username, consoleInfo.Password, consoleInfo.Ip, consoleInfo.Port)
		}
		return nil
	})
	return url, err
}

func (solus *SolusVM) vmTunTapFunc(id int) func(value string) error {
	return func(value string) error {
		return solus.Api.VmTunTap(id, value == "enable")
	}
}

func (solus *SolusVM) RenameInstance(instanceID string, name string) error {
	return solus.instanceAction(instanceID, func(id int) error {
		return solus.Api.VmHostname(id, name)
	})
}

func (solus *SolusVM) ReimageInstance(instanceID string, image *compute.Image) error {
	return solus.instanceAction(instanceID, func(id int) error {
		return solus.Api.VmReimage(id, image.ID)
	})
}

func (solus *SolusVM) ResizeInstance(instanceID string, flavor *compute.Flavor) error {
	return solus.instanceAction(instanceID, func(id int) error {
		// we start with disks ince that is the most likely one to have problem
		if flavor.DiskGB != 0 {
			err := solus.Api.VmResizeDisk(id, flavor.DiskGB)
			if err != nil {
				return err
			}
		}
		if flavor.MemoryMB != 0 {
			err := solus.Api.VmResizeMemory(id, flavor.MemoryMB)
			if err != nil {
				return err
			}
		}
		if flavor.NumCores != 0 {
			err := solus.Api.VmResizeCpu(id, flavor.NumCores)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (solus *SolusVM) ListInstanceAddresses(instanceID string) ([]*compute.Address, error) {
	var addresses []*compute.Address
	err := solus.instanceAction(instanceID, func(id int) error {
		apiInfo, err := solus.Api.VmInfo(id)
		if err != nil {
			return err
		}

		for _, addrString := range strings.Split(apiInfo.Ips, ",") {
			addresses = append(addresses, &compute.Address{
				ID:        instanceID + ":" + addrString,
				IP:        addrString,
			})
		}
		return nil
	})
	return addresses, err
}

func (solus *SolusVM) AddAddressToInstance(instanceID string, address *compute.Address) error {
	return solus.instanceAction(instanceID, solus.Api.VmAddAddress)
}

func (solus *SolusVM) RemoveAddressFromInstance(instanceID string, addressID string) error {
	addrs, err := solus.ListInstanceAddresses(instanceID)
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

	return solus.instanceAction(instanceID, func(id int) error {
		return solus.Api.VmRemoveAddress(id, addr.IP)
	})
}

func (solus *SolusVM) SetAddressHostname(addressID string, hostname string) error {
	return fmt.Errorf("operation not supported")
}
