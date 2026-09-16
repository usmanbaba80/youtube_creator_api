package security

import (
	"crypto/subtle"
	"strings"
)

// ValidAPIKey reports whether provided matches any configured key using
// constant-time comparison to reduce timing side channels.
func ValidAPIKey(provided string, keys []string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" || len(keys) == 0 {
		return false
	}
	providedBytes := []byte(provided)
	ok := false
	for _, key := range keys {
		candidate := []byte(strings.TrimSpace(key))
		if subtle.ConstantTimeCompare(providedBytes, candidate) == 1 {
			ok = true
		}
	}
	return ok
}
