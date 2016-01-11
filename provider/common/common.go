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
