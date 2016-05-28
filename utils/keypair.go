package utils

import "golang.org/x/crypto/ssh"

import "encoding/base64"
import "fmt"
import "strings"

// Converts a public key to authorized keys format.
func PublicKeyToAuthorizedKeysFormat(in string) (string, error) {
	s := strings.Replace(strings.Replace(strings.TrimSpace(in), "\r\n", "\n", -1), "\r", "\n", -1)
	lines := strings.Split(s, "\n")
	if len(lines) == 1 {
		publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(lines[0]))
		if err != nil {
			return "", fmt.Errorf("failed to parse public key from authorized keys format: %v", err)
		}
		encodedKey := base64.StdEncoding.EncodeToString(publicKey.Marshal())
		return fmt.Sprintf("%s %s %s", publicKey.Type(), encodedKey, comment), nil
	} else {
		var encodedKey string
		continuationLine := false
		for _, line := range lines {
			if continuationLine || strings.ContainsAny(line, ":-") {
				continuationLine = strings.HasSuffix(line, "\\")
			} else {
				encodedKey += line
			}
		}
		rawKey, err := base64.StdEncoding.DecodeString(encodedKey)
		fmt.Println(encodedKey)
		if err != nil {
			return "", fmt.Errorf("detected SSH2 format, but contains invalid base64 content: %v", err)
		}
		fmt.Println(string(base64.StdEncoding.EncodeToString(rawKey)))
		publicKey, err := ssh.ParsePublicKey(rawKey)
		if err != nil {
			return "", fmt.Errorf("failed to parse public key from SSH2 format: %v", err)
		}
		newEncodedKey := base64.StdEncoding.EncodeToString(publicKey.Marshal())
		return fmt.Sprintf("%s %s cloug@cloug", publicKey.Type(), newEncodedKey), nil
	}
}
