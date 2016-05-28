package compute

type Instance struct {
	ID        string
	Name      string
	Region    string
	IP        string
	PrivateIP string

	Image     Image
	Flavor    Flavor
	PublicKey PublicKey

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

	// Additional custom actions supported for this instance.
	Actions []InstanceAction
}

type InstanceStatus string

const (
	StatusOnline  InstanceStatus = "online"
	StatusOffline InstanceStatus = "offline"
)

type InstanceAction struct {
	Label       string
	Description string

	// map of options from option value to description
	// if nil, then this action can take any string value
	Options map[string]string

	Func func(string) error
}
