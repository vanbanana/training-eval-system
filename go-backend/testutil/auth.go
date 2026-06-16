// Package testutil provides test helpers for integration tests.
package testutil

import (
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
)

const TestJWTSecret = "test-jwt-secret-key-for-testing-purposes-32chars-min"

// GenerateTestToken creates a JWT token for testing with the given user info.
func GenerateTestToken(userID int64, username, role string) string {
	claims := &crypto.Claims{
		Sub:      userID,
		Username: username,
		Role:     role,
		Type:     "access",
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}
	token, _ := crypto.SignToken(TestJWTSecret, claims)
	return token
}

// AdminToken returns a valid admin JWT token.
func AdminToken() string {
	return GenerateTestToken(1, "admin", "admin")
}

// TeacherToken returns a valid teacher JWT token.
func TeacherToken() string {
	return GenerateTestToken(2, "teacher1", "teacher")
}

// StudentToken returns a valid student JWT token.
func StudentToken() string {
	return GenerateTestToken(3, "student1", "student")
}

// testMasterKey returns a fixed 32-byte AES key for tests.
func testMasterKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}
