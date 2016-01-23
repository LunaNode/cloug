package api

type ErrorResponse struct {
	ErrorCode int    `json:"errorcode"`
	ErrorText string `json:"errortext"`
}

type IDResponse struct {
	ID string `json:"id"`
}

type ServiceOffering struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CPUNumber int    `json:"cpunumber"`
	Memory    int    `json:"memory"`
}

type ListServiceOfferingsResponse struct {
	ServiceOfferings []ServiceOffering `json:"serviceoffering"`
}

type DiskOffering struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DiskSize     int    `json:"disksize"`
	IsCustomized bool   `json:"iscustomized"`
}

type ListDiskOfferingsResponse struct {
	DiskOfferings []DiskOffering `json:"diskoffering"`
}

type Nic struct {
	Addr string `json:"ipaddress"`
}

type DeployVirtualMachineOptions struct {
	// Required options
	ServiceOffering string
	DiskOffering    string
	Template        string

	// Optional options
	Network string
	Name    string
}

type DeployVirtualMachineResponse struct {
	ID    string `json:"id"`
	JobID string `json:"jobid"`
}

type DeployVirtualMachineResult struct {
	Password string `json:"password"`
}

type VirtualMachine struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Nics     []Nic  `json:"nic"`
	Hostname string `json:"hostname"`
}

type ListVirtualMachinesResponse struct {
	VirtualMachines []VirtualMachine `json:"virtualmachine"`
}
