package compute

type Address struct {
	ID        string
	IP        string
	PrivateIP string

	CanDNS   bool
	Hostname string
}
