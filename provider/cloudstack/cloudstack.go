package cloudstack

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/provider/cloudstack/api"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "errors"
import "fmt"
import "strings"

type CloudStack struct {
	client *api.API
}

func MakeCloudStack(targetURL string, zoneID string, apiKey string, secretKey string) *CloudStack {
	cs := new(CloudStack)
	cs.client = &api.API{
		TargetURL: targetURL,
		ZoneID:    zoneID,
		APIKey:    apiKey,
		SecretKey: secretKey,
	}
	return cs
}

func (cs *CloudStack) ComputeService() compute.Service {
	return cs
}

func (cs *CloudStack) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "Running" {
		return compute.StatusOnline
	} else if status == "Stopped" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (cs *CloudStack) vmToInstance(vm *api.VirtualMachine) *compute.Instance {
	instance := &compute.Instance{
		ID:     vm.ID,
		Name:   vm.Hostname,
		Status: cs.mapInstanceStatus(vm.State),
	}

	for _, nic := range vm.Nics {
		if utils.IsPrivate(nic.Addr) {
			instance.PrivateIP = nic.Addr
		} else {
			instance.IP = nic.Addr
		}
	}

	return instance
}

func (cs *CloudStack) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	if instance.Image.ID == "" {
		return nil, errors.New("instance image ID must be set")
	}

	flavorID, err := common.GetMatchingFlavorID(cs, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(flavorID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid service / disk offering: %s", flavorID)
	}

	opts := api.DeployVirtualMachineOptions{
		ServiceOffering: parts[0],
		DiskOffering:    parts[1],
		Template:        instance.Image.ID,
		Network:         instance.NetworkID,
		Name:            instance.Name,
	}

	id, jobid, err := cs.client.DeployVirtualMachine(&opts)
	if err != nil {
		return nil, err
	}

	return &compute.Instance{
		ID:    id,
		Name:  instance.Name,
		JobID: jobid,
	}, nil
}

func (cs *CloudStack) DeleteInstance(instanceID string) error {
	return cs.client.DestroyVirtualMachine(instanceID, true)
}

func (cs *CloudStack) ListInstances() ([]*compute.Instance, error) {
	vms, err := cs.client.ListVirtualMachines()
	if err != nil {
		return nil, err
	}

	instances := make([]*compute.Instance, len(vms))
	for i := range vms {
		instances[i] = cs.vmToInstance(&vms[i])
	}
	return instances, nil
}

func (cs *CloudStack) GetInstance(instanceID string) (*compute.Instance, error) {
	vm, err := cs.client.GetVirtualMachine(instanceID)
	if err != nil {
		return nil, err
	}
	return cs.vmToInstance(vm), nil
}

func (cs *CloudStack) StartInstance(instanceID string) error {
	return cs.client.StartVirtualMachine(instanceID)
}

func (cs *CloudStack) StopInstance(instanceID string) error {
	return cs.client.StopVirtualMachine(instanceID)
}

func (cs *CloudStack) RebootInstance(instanceID string) error {
	return cs.client.RebootVirtualMachine(instanceID)
}

func (cs *CloudStack) ListFlavors() ([]*compute.Flavor, error) {
	return nil, errors.New("operation not supported")
}

func (cs *CloudStack) findServiceOffering(cpu int, ram int) (string, error) {
	offerings, err := cs.client.ListServiceOfferings()
	if err != nil {
		return "", err
	}
	for _, offering := range offerings {
		if offering.CPUNumber == cpu && offering.Memory == ram {
			return offering.ID, nil
		}
	}
	return "", fmt.Errorf("no service offering with %d vcpus and %d MB RAM", cpu, ram)
}

func (cs *CloudStack) findDiskOffering(size int) (string, error) {
	offerings, err := cs.client.ListDiskOfferings()
	if err != nil {
		return "", err
	}
	for _, offering := range offerings {
		if offering.DiskSize == size {
			return offering.ID, nil
		}
	}
	return "", fmt.Errorf("no disk offering with %d GB space", size)
}

func (cs *CloudStack) FindFlavor(flavor *compute.Flavor) (string, error) {
	cpu := flavor.NumCores
	ram := flavor.MemoryMB
	disk := flavor.DiskGB

	if cpu == 0 {
		cpu = 1
	}
	if ram == 0 {
		ram = 512
	}
	if disk == 0 {
		disk = 10
	}

	serviceOffering, err := cs.findServiceOffering(cpu, ram)
	if err != nil {
		return "", nil
	}

	diskOffering, err := cs.findDiskOffering(disk)
	if err != nil {
		return "", nil
	}

	return fmt.Sprintf("%s/%s", serviceOffering, diskOffering), nil
}
