package api

const LNDYNAMIC_API_URL = "https://dynamic.lunanode.com/api/{CATEGORY}/{ACTION}/"

type GenericResponse struct {
	Success string `json:"success"`
	Error   string `json:"error"`
}

// virtual machines

type VmCreateResponse struct {
	ID string `json:"vm_id"`
}

type VmVncResponse struct {
	VncUrl string `json:"vnc_url"`
}

type VmStruct struct {
	ID        string `json:"vm_id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	Hostname  string `json:"hostname"`
	PrimaryIP string `json:"primaryip"`
	PrivateIP string `json:"privateip"`
	PlanID    string `json:"plan_id"`
	RAM       string `json:"ram"`
	Vcpu      string `json:"vcpu"`
	Storage   string `json:"storage"`
	Bandwidth string `json:"bandwidth"`
}

type VmInfoStruct struct {
	IP            string `json:"ip"`
	PrivateIP     string `json:"privateip"`
	Status        string `json:"status_nohtml"`
	Hostname      string `json:"hostname"`
	BandwidthUsed string `json:"bandwidthUsedGB"`
	LoginDetails  string `json:"login_details"`
	DiskSwap      string `json:"diskswap"`
}

type VmListResponse struct {
	Vms []VmStruct `json:"vms"`
}

type VmInfoResponse struct {
	Info  *VmInfoStruct `json:"info"`
	Extra *VmStruct     `json:"extra"`
}

// image

type Image struct {
	ID     string `json:"image_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Size   string `json:"size"`
	Region string `json:"region"`
}

type ImageListResponse struct {
	Images []*Image `json:"images"`
}

type ImageDetailsResponse struct {
	Image *Image `json:"details"`
}

type ImageCreateResponse struct {
	ID string `json:"image_id"`
}

// volumes

type Volume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Size   string `json:"size"`
	Region string `json:"region"`
	Status string `json:"status"`
}

type VolumeListResponse struct {
	Volumes []*Volume `json:"volumes"`
}

type VolumeInfoResponse struct {
	Volume *Volume `json:"volume"`
}

type VolumeCreateResponse struct {
	ID string `json:"volume_id"`
}

// plans

type Plan struct {
	ID        string `json:"plan_id"`
	Name      string `json:"name"`
	Vcpu      string `json:"vcpu"`
	Price     string `json:"price"`
	RAM       string `json:"ram"`
	Storage   string `json:"storage"`
	Bandwidth string `json:"bandwidth"`
}

type PlanListResponse struct {
	Plans []*Plan `json:"plans"`
}
