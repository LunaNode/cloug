package utils

import "crypto/rand"

func Uid(l int) string {
	return UidAlphabet(l, []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"))
}

func UidAlphabet(l int, alphabet []rune) string {
	bytes := make([]byte, l)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	str := make([]rune, len(bytes))
	for i := range bytes {
		str[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return string(str)
}

func IsSliceSubset(set []string, subset []string) bool {
	setMap := make(map[string]bool)
	for _, str := range set {
		setMap[str] = true
	}
	for _, str := range subset {
		if !setMap[str] {
			return false
		}
	}
	return true
}
