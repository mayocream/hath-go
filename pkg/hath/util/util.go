package util

import (
	"crypto/sha1"
	"encoding/hex"
	"time"
)

// SystemTime ...
func SystemTime() int {
	return int(time.Now().Unix())
}

// SHA1 ...
func SHA1(str string) string {
	s := sha1.Sum([]byte(str))
	return hex.EncodeToString(s[:])
}
