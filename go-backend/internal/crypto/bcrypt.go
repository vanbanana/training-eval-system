package crypto

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

// HashPassword hashes a plaintext password using bcrypt with cost 10.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("crypto: hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks if a plaintext password matches a bcrypt hash.
// Returns nil on success, error on mismatch or failure.
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
