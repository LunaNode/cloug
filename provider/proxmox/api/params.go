package api

type AuthenticateResponse struct {
	CSRFPreventionToken string `json:"CSRFPreventionToken"`
	Ticket              string `json:"ticket"`
	Username            string `json:"username"`
}

type Node struct {
	Node string `json:"node"`

	Memory    int64 `json:"mem"`
	MaxMemory int64 `json:"maxmem"`

	Disk    int64 `json:"disk"`
	MaxDisk int64 `json:"maxdisk"`
}

type VM struct {
	ID     int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	CPUs   int    `json:"cpus"`

	Memory    int64 `json:"mem"`
	MaxMemory int64 `json:"maxmem"`

	Disk    int64 `json:"disk"`
	MaxDisk int64 `json:"maxdisk"`
}

type CreateVMOptions struct {
	ID   int
	Node string
	Name string

	// Number of cores per socket
	Cores int

	// Amount of RAM in MB
	Memory int

	// Operating system type, to enable special optimization/features for specific OS
	OSType OSType

	// ISO identifier, like "local:iso/ubuntu.iso"
	ISO string

	// Storage destination, defaults to "local"
	Storage string

	// Disk capacity in GB
	DiskSize int

	// Network bridge name, if empty then uses default NAT setup
	NetworkBridge string

	// Network driver, defaults to e1000
	NetworkDriver NetworkDriver
}

type OSType string

const (
	OtherOS        OSType = "other"
	MSWindowsXP           = "wxp"
	MSWindows2000         = "w2k"
	MSWindows2003         = "w2k3"
	MSWindows2008         = "w2k8"
	MSWindowsVista        = "wvista"
	MSWindows7            = "win7"
	MSWindows8            = "win8"
	Linux24               = "l24"
	Linux26               = "l26"
	Solaris               = "solaris"
)

type NetworkDriver string

const (
	E1000     NetworkDriver = "e1000"
	VirtioNet               = "virtio"
)
