package auth

import (
	"crypto/subtle"
	"strings"
)

// splitHash splits an encoded Argon2id hash into its parts:
// ["$argon2id", "v=19", "m=...,t=...,p=...", salt, hash].
func splitHash(encoded string) []string {
	parts := strings.Split(encoded, "$")
	// Leading empty string because the format starts with "$".
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil
	}
	return parts
}

// subtleEqual compares two byte slices in constant time.
func subtleEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
