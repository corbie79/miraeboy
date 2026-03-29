package storage

import (
	"crypto/sha256"
	"fmt"
)

// HashPassword returns a hex-encoded SHA-256 hash of the password.
// Simple but sufficient for an internal tool; upgrade to bcrypt if needed.
func HashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", sum)
}
