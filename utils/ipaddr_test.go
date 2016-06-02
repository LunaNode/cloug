package utils

import "testing"

func TestIsPrivate(t *testing.T) {
	m := map[string]bool{
		"0.0.0.0":         false,
		"255.255.255.255": false,
		"1.2.3.4":         false,
		"10.0.0.0":        true,
		"10.255.255.255":  true,
		"172.20.30.40":    true,
		"192.168.100.5":   true,
	}

	for ip, rv := range m {
		if IsPrivate(ip) != rv {
			t.Fatalf("IsPrivate(%s) = %t, expected %t", ip, IsPrivate(ip), rv)
		}
	}
}

func TestGetIPVersion(t *testing.T) {
	m := map[string]int{
		"":                                        0,
		"hello":                                   0,
		"1.2.3":                                   0,
		"0.0.0.0":                                 4,
		"1.2.3.4":                                 4,
		"1.2.3.256":                               0,
		"123.255.123.255":                         4,
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334": 6,
		"2001:0db8:85a3::8a2e:0370:7334":          6,
	}

	for ip, rv := range m {
		if GetIPVersion(ip) != rv {
			t.Fatalf("GetIPVersion(%s) = %d, expected %d", ip, GetIPVersion(ip), rv)
		}
	}
}
