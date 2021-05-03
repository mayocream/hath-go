package util

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
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

// ParseAddition ...
func ParseAddition(add string) map[string]string {
	var kvs map[string]string
	if len(add) == 0 {
		kvs = make(map[string]string)
		return kvs
	}

	list := strings.Split(add, ";")
	kvs = make(map[string]string, len(list))
	for _, str := range list {
		kv := strings.Split(str, "=")
		if len(kv) == 2 {
			kvs[strings.Trim(kv[0], " ")] = strings.Trim(kv[1], " ")
		}
	}

	return kvs
}
