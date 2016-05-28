package common

import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "fmt"

func GetMatchingImageID(service compute.ImageService, image *compute.Image) (string, error) {
	if image.ID == "" {
		imageID, err := service.FindImage(image)
		if err != nil {
			return "", fmt.Errorf("error finding image: %v", err)
		} else if imageID == "" {
			return "", fmt.Errorf("no matching image found")
		} else {
			return imageID, nil
		}
	} else {
		return image.ID, nil
	}
}

func GetMatchingFlavorID(service compute.FlavorService, flavor *compute.Flavor) (string, error) {
	if flavor.ID == "" {
		flavorID, err := service.FindFlavor(flavor)
		if err != nil {
			return "", fmt.Errorf("error finding flavor: %v", err)
		} else if flavorID == "" {
			return "", fmt.Errorf("no matching flavor found")
		} else {
			return flavorID, nil
		}
	} else {
		return flavor.ID, nil
	}
}

// Attempts to find a flavor in options that is similar to target.
// Returns flavor ID if successful, or empty string if no suitable flavor found.
func MatchFlavor(target *compute.Flavor, options []*compute.Flavor) string {
	for _, option := range options {
		if target.Name != "" && target.Name != option.Name {
			continue
		} else if len(option.Regions) > 0 && len(target.Regions) > 0 && !utils.IsSliceSubset(option.Regions, target.Regions) {
			continue
		} else if target.NumCores != 0 && target.NumCores != option.NumCores {
			continue
		} else if target.DiskGB != 0 && target.DiskGB != option.DiskGB {
			continue
		} else if target.MemoryMB != 0 && target.MemoryMB != option.MemoryMB {
			continue
		} else if target.TransferGB != 0 && target.TransferGB != option.TransferGB {
			continue
		}
		return option.ID
	}

	return ""
}

// Wraps VM creation with a raw SSH key for KeypairServices.
// Calls ImportPublicKey, then CreateInstance, and finally RemovePublicKey.
func KeypairServiceCreateWrapper(service compute.Service, keypair compute.KeypairService, instance *compute.Instance) (*compute.Instance, error) {
	// wrapper should only be used if an instance is provided with a raw key
	if instance.PublicKey.ID != "" {
		return nil, fmt.Errorf("instance has non-empty public key ID, but service called wrapper")
	} else if len(instance.PublicKey.Key) == 0 {
		return nil, fmt.Errorf("instance does not specify a public key, but service called wrapper")
	}

	// proceed with import/create/remove operation
	publicKey, err := keypair.ImportPublicKey(&instance.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to import public key: %v", err)
	}
	wrappedInstance := *instance
	wrappedInstance.PublicKey = *publicKey
	createdInstance, err := service.CreateInstance(&wrappedInstance)
	keypair.RemovePublicKey(publicKey.ID)
	return createdInstance, err
}

const DEFAULT_PUBLIC_KEY_LABEL = "cloug"

func ImportPublicKeyWrapper(key *compute.PublicKey, f func(string, string) (string, error)) (*compute.PublicKey, error) {
	if len(key.Key) == 0 {
		return nil, fmt.Errorf("key content is empty")
	}

	label := key.Label
	if label == "" {
		label = DEFAULT_PUBLIC_KEY_LABEL
	}

	publicKey, err := utils.PublicKeyToAuthorizedKeysFormat(string(key.Key))
	if err != nil {
		return nil, fmt.Errorf("failed to convert provided key to authorized_keys format: %v", err)
	}

	id, err := f(label, publicKey)
	if err != nil {
		return nil, err
	}
	return &compute.PublicKey{
		ID:    id,
		Label: label,
		Key:   []byte(publicKey),
	}, nil
}
