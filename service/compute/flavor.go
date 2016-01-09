package compute

type Flavor struct {
	ID      string
	Name    string
	Regions []string

	NumCores   int
	DiskGB     int
	MemoryMB   int
	TransferGB int
}
