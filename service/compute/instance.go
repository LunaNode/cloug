package compute

type Instance struct {
	ID        string
	Name      string
	Region    string
	IP        string
	PrivateIP string

	Image  Image
	Flavor Flavor

	Status InstanceStatus

	Username string
	Password string

	// A temporary identifier that can be used to get additional details on the
	// instance immediately after it has been provisioned. For example, some
	// providers return the instance password this way.
	JobID string

	// Bytes of bandwidth used.
	BandwidthUsed int64

	// Tenant network ID.
	NetworkID string

	// Key-value additional details of the instance.
	Details map[string]string
}

type InstanceStatus string

const (
	StatusOnline  InstanceStatus = "online"
	StatusOffline InstanceStatus = "offline"
)
