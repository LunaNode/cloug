package compute

type Provider interface {
	ComputeService() Service
}

type Service interface {
	// Creates a new instance.
	// All fields in Instance struct are optional. ID and JobID are always ignored.
	// Other fields may also be ignored or cause errors.
	CreateInstance(instance *Instance) (*Instance, error)

	// Destroys the specified instance.
	DeleteInstance(instanceID string) error

	// Lists instances.
	ListInstances() ([]*Instance, error)

	// Gets information about a specific instance.
	// This may return more details than ListInstances.
	GetInstance(instanceID string) (*Instance, error)

	// Starts/boots an instance.
	StartInstance(instanceID string) error

	// Stops/shuts down an instance.
	StopInstance(instanceID string) error

	// Reboots an instance.
	RebootInstance(instanceID string) error
}

type VNCService interface {
	// If successful, returns a string URL where VNC or similar console for the instance can be accessed.
	GetVNC(instanceID string) (string, error)
}

type RenameService interface {
	RenameInstance(instanceID string, name string) error
}

type ReimageService interface {
	ReimageInstance(instanceID string, image *Image) error
}

type ResizeService interface {
	ResizeInstance(instanceID string, flavor *Flavor) error
}

type ImageService interface {
	// Creates an image based on the specified fields.
	// If SourceInstance is set, the instance is snapshotted
	// If SourceURL is set, the image is retrieved from URL.
	CreateImage(image *Image) (*Image, error)

	// Looks for an image matching the specified image.
	// If successful, returns nil if image not found, or the image ID.
	FindImage(image *Image) (string, error)

	ListImages() ([]*Image, error)
	GetImage(imageID string) (*Image, error)
	DeleteImage(imageID string) error
}

type AddressService interface {
	ListInstanceAddresses(instanceID string) ([]*Address, error)
	AddAddressToInstance(instanceID string, address *Address) error
	RemoveAddressFromInstance(instanceID string, addressID string) error
	SetAddressHostname(addressID string, hostname string) error
}

type FlavorService interface {
	ListFlavors() ([]*Flavor, error)
	FindFlavor(flavor *Flavor) (string, error)
}
