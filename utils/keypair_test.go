package utils

import "strings"
import "testing"

func Test_Keypair_PublicKeyToAuthorizedKeysFormat_RSA(t *testing.T) {
	in := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC4b+H5kTHuOtXjLlTsOMQmRu9zagZVxYoVv3QQGGrDWWKFUrQlKRJmZ0M1WYVvnODyufbtiT++snsNglMKuXyf3fvljSd1KaFDaxkxiZ7sGK7EUeXx7g3/tq3/x6BWyKCP/97HBtc0PVLuYftEI32nqRfwZFHPKVH7Fe0k+TNtPjs0xg6QXrC0Lh1E9NPZ3qWHgO6OkWlver4B6nDH/BIRKxp0N7+nROdV2i3ivUSHdk9nl08zxHJzwIFtojhqbRNl0tRgLvD8cTEnIw4ELz5OJP+XBWgnpnsBzJielCqHxXKgAXDX+jfhsfrpxpDqtJ5Gh6wae3gtkFLJqwx/Xy2N blah@example.com"
	out, err := PublicKeyToAuthorizedKeysFormat(in)
	if err != nil {
		t.Fatalf("returned error: %v", err)
	} else if out != in {
		t.Fatalf("output key does not match original: %s", out)
	}
}

func Test_Keypair_PublicKeyToAuthorizedKeysFormat_SSH2(t *testing.T) {
	in := `---- BEGIN SSH2 PUBLIC KEY ----
Comment: "1024-bit RSA, converted from OpenSSH by me@example.com"
x-command: /home/me/bin/lock-in-guest.sh
AAAAB3NzaC1yc2EAAAABIwAAAIEA1on8gxCGJJWSRT4uOrR13mUaUk0hRf4RzxSZ1zRb
YYFw8pfGesIFoEuVth4HKyF8k1y4mRUnYHP1XNMNMJl1JcEArC2asV8sHf6zSPVffozZ
5TT4SfsUu/iKy9lUcCfXzwre4WWZSXXcPff+EHtWshahu3WzBdnGxm5Xoi89zcE=
---- END SSH2 PUBLIC KEY ----`
	expected := "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAIEA1on8gxCGJJWSRT4uOrR13mUaUk0hRf4RzxSZ1zRbYYFw8pfGesIFoEuVth4HKyF8k1y4mRUnYHP1XNMNMJl1JcEArC2asV8sHf6zSPVffozZ5TT4SfsUu/iKy9lUcCfXzwre4WWZSXXcPff+EHtWshahu3WzBdnGxm5Xoi89zcE= "
	out, err := PublicKeyToAuthorizedKeysFormat(in)
	if err != nil {
		t.Fatalf("returned error: %v", err)
	} else if !strings.HasPrefix(out, expected) {
		t.Fatalf("output key does not match expected: %s", out)
	}
}
