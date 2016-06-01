package googlecompute

import "strings"

func basename(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
