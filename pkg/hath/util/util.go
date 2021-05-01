package util

import (
	"crypto/sha1"
	"time"
)

// SystemTime ...
func SystemTime() int {
	return time.Now().Second()
}

// SHA1 ...
func SHA1(str string) string {
	s := sha1.Sum([]byte(str))
	return string(s[:])
}
