package compute

type Image struct {
	ID      string
	Name    string
	Regions []string
	Type    ImageType
	Format  string
	Status  ImageStatus
	Public  bool
	Size    int64

	SourceInstance string
	SourceURL      string

	Distribution string
	Version      string
	Architecture ImageArchitecture

	// Key-value additional details of the image.
	Details map[string]string
}

type ImageType string

const (
	TemplateImage ImageType = "template"
	ISOImage                = "iso"
)

type ImageStatus string

const (
	ImageAvailable ImageStatus = "available"
	ImagePending   ImageStatus = "pending"
)

type ImageArchitecture string

const (
	ArchAMD64 = "amd64"
	Archi386  = "i386"
)
