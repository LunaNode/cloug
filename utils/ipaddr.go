package utils

import "net"

var privateNetworks []*net.IPNet

func Init() {
	privateNetworks = make([]*net.IPNet, 0)
	for _, cidr := range []string{"192.168.0.0/16", "172.16.0.0/12", "10.0.0.0/8"} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		privateNetworks = append(privateNetworks, network)
	}
}

func IsPrivate(ipString string) bool {
	if privateNetworks == nil {
		Init()
	}

	ip := net.ParseIP(ipString)
	for _, net := range privateNetworks {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// Returns the IP version of the specified IP address (e.g. 4 for IPv4, 6 for IPv6),
//  or 0 if the address is invalid.
func GetIPVersion(ipString string) int {
	ip := net.ParseIP(ipString)
	if ip == nil {
		return 0
	}
	if ip.To4() != nil {
		return 4
	} else {
		return 6
	}
}
