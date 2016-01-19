package proxmox

import "github.com/LunaNode/cloug/provider/proxmox/api"
import "github.com/LunaNode/cloug/service/compute"

import "errors"
import "fmt"
import "math/rand"
import "strconv"
import "strings"

const DEFAULT_DISK = 32
const DEFAULT_MEMORY = 512

type Proxmox struct {
	Client *api.API
}

func MakeProxmox(baseURL string, username string, password string) *Proxmox {
	pm := new(Proxmox)
	pm.Client = api.NewAPI(baseURL, username, password)
	return pm
}

func (pm *Proxmox) ComputeService() compute.Service {
	return pm
}

func (pm *Proxmox) mapInstanceStatus(status string) compute.InstanceStatus {
	if status == "running" {
		return compute.StatusOnline
	} else if status == "stopped" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(status))
	}
}

func (pm *Proxmox) vmToInstance(vm *api.VM, node string) *compute.Instance {
	return &compute.Instance{
		ID:     fmt.Sprintf("%s/%d", node, vm.ID),
		Name:   vm.Name,
		Status: pm.mapInstanceStatus(vm.Status),
	}
}

func (pm *Proxmox) splitInstanceID(id string) (string, int, error) {
	// id is like "node/vmid"
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid id: %s", id)
	}
	vmid, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid vmid: %s", parts[1])
	}
	return parts[0], vmid, nil
}

func (pm *Proxmox) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	if instance.Image.ID == "" {
		return nil, errors.New("image ID must be specified")
	}

	targetDisk := instance.Flavor.DiskGB
	if targetDisk == 0 {
		targetDisk = DEFAULT_DISK
	}
	targetMemory := instance.Flavor.MemoryMB
	if targetMemory == 0 {
		targetMemory = DEFAULT_MEMORY
	}

	// find a node to provision on
	// currently we just pick the one with sufficient disk space that has most free memory
	nodes, err := pm.Client.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("error listing nodes: %v", err)
	}

	var bestNode string
	var bestMemory int64

	for _, node := range nodes {
		if node.MaxDisk-node.Disk < int64(targetDisk)*1024*1024*1024 {
			continue
		}
		freeMemory := node.MaxMemory - node.Memory
		if freeMemory > bestMemory {
			bestNode = node.Node
			bestMemory = freeMemory
		}
	}

	if bestNode == "" {
		return nil, fmt.Errorf("no node found with sufficient resources")
	}

	// find free ID to use on this node
	var freeID int

	vms, err := pm.Client.ListVMsOnNode(bestNode)
	if err != nil {
		return nil, fmt.Errorf("error listing vms on %s: %v", bestNode, err)
	}
	ids := make(map[int]bool)
	for _, vm := range vms {
		ids[vm.ID] = true
	}

	for _, i := range rand.Perm(899) {
		id := i + 100
		if !ids[id] {
			freeID = id
			break
		}
	}

	if freeID == 0 {
		return nil, fmt.Errorf("did not find any free ID on node %s", bestNode)
	}

	// create the VM
	opts := api.CreateVMOptions{
		ID:       freeID,
		Node:     bestNode,
		Name:     instance.Name,
		Cores:    instance.Flavor.NumCores,
		Memory:   targetMemory,
		OSType:   api.Linux26,
		ISO:      instance.Image.ID,
		Storage:  "local",
		DiskSize: targetDisk,
	}

	if opts.Cores == 0 {
		opts.Cores = 1
	}

	err = pm.Client.CreateVM(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM on node %s: %v", bestNode, err)
	}

	return &compute.Instance{
		ID:   fmt.Sprintf("%s/%d", bestNode, freeID),
		Name: instance.Name,
	}, nil
}

func (pm *Proxmox) instanceAction(instanceID string, f func(node string, vmid int) error) error {
	node, vmid, err := pm.splitInstanceID(instanceID)
	if err != nil {
		return err
	} else {
		return f(node, vmid)
	}
}

func (pm *Proxmox) DeleteInstance(instanceID string) error {
	return pm.instanceAction(instanceID, pm.Client.DeleteVM)
}

func (pm *Proxmox) ListInstances() ([]*compute.Instance, error) {
	nodes, err := pm.Client.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	var instances []*compute.Instance

	for _, node := range nodes {
		vms, err := pm.Client.ListVMsOnNode(node.Node)
		if err != nil {
			return nil, fmt.Errorf("failed to list VMs on node %s: %v", node.Node, err)
		}
		for _, vm := range vms {
			instances = append(instances, pm.vmToInstance(&vm, node.Node))
		}
	}

	return instances, nil
}

func (pm *Proxmox) GetInstance(instanceID string) (*compute.Instance, error) {
	node, vmid, err := pm.splitInstanceID(instanceID)
	if err != nil {
		return nil, err
	}

	vm, err := pm.Client.GetVMStatus(node, vmid)
	if err != nil {
		return nil, err
	} else {
		return pm.vmToInstance(vm, node), nil
	}
}

func (pm *Proxmox) StartInstance(instanceID string) error {
	return pm.instanceAction(instanceID, pm.Client.StartVM)
}

func (pm *Proxmox) StopInstance(instanceID string) error {
	return pm.instanceAction(instanceID, pm.Client.StopVM)
}

func (pm *Proxmox) RebootInstance(instanceID string) error {
	return pm.instanceAction(instanceID, pm.Client.ResetVM)
}
