package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 5: JWT sign/verify round-trip
func TestProperty_JWTRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		secret := rapid.StringMatching(`[a-zA-Z0-9]{32,64}`).Draw(t, "secret")
		userID := rapid.Int64Range(1, 999999).Draw(t, "userID")
		username := rapid.StringMatching(`[a-z]{3,20}`).Draw(t, "username")
		role := rapid.SampledFrom([]string{"admin", "teacher", "student"}).Draw(t, "role")

		claims := &Claims{
			Sub:      userID,
			Username: username,
			Role:     role,
			Type:     "access",
			Iat:      time.Now().Unix(),
			Exp:      time.Now().Add(1 * time.Hour).Unix(),
		}

		token, err := SignToken(secret, claims)
		if err != nil {
			t.Fatalf("SignToken failed: %v", err)
		}

		got, err := VerifyToken(secret, token)
		if err != nil {
			t.Fatalf("VerifyToken failed: %v", err)
		}

		if got.Sub != claims.Sub {
			t.Fatalf("Sub mismatch: got %d, want %d", got.Sub, claims.Sub)
		}
		if got.Username != claims.Username {
			t.Fatalf("Username mismatch: got %q, want %q", got.Username, claims.Username)
		}
		if got.Role != claims.Role {
			t.Fatalf("Role mismatch: got %q, want %q", got.Role, claims.Role)
		}
		if got.Type != claims.Type {
			t.Fatalf("Type mismatch: got %q, want %q", got.Type, claims.Type)
		}
	})
}

// Property 6: Password hash/verify round-trip
func TestProperty_PasswordRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// bcrypt max is 72 bytes
		passLen := rapid.IntRange(1, 72).Draw(t, "passLen")
		password := rapid.StringOfN(rapid.Rune(), passLen, passLen, -1).Draw(t, "password")
		// Ensure it's ASCII-safe for bcrypt
		if len([]byte(password)) > 72 {
			password = password[:72]
		}

		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		// Same password should verify
		if err := VerifyPassword(hash, password); err != nil {
			t.Fatalf("VerifyPassword failed for same password: %v", err)
		}

		// Different password should fail
		different := password + "x"
		if len(different) > 72 {
			different = password[:len(password)-1] + "y"
		}
		if err := VerifyPassword(hash, different); err == nil {
			t.Fatalf("VerifyPassword should fail for different password")
		}
	})
}

// Property 12: AES-256-GCM encryption round-trip
func TestProperty_AESRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random 32-byte key
		key := make([]byte, 32)
		rand.Read(key)

		// Generate random plaintext (0-1000 bytes)
		ptLen := rapid.IntRange(0, 1000).Draw(t, "ptLen")
		plaintext := make([]byte, ptLen)
		rand.Read(plaintext)

		// Encrypt
		ciphertext, err := Encrypt(key, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		// Decrypt with same key
		decrypted, err := Decrypt(key, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if len(decrypted) != len(plaintext) {
			t.Fatalf("length mismatch: got %d, want %d", len(decrypted), len(plaintext))
		}
		for i := range plaintext {
			if decrypted[i] != plaintext[i] {
				t.Fatalf("byte mismatch at index %d", i)
			}
		}

		// Decrypt with different key should fail
		wrongKey := make([]byte, 32)
		rand.Read(wrongKey)
		_, err = Decrypt(wrongKey, ciphertext)
		if err == nil {
			t.Fatalf("Decrypt with wrong key should fail")
		}
	})
}

// Property 5 supplement: DeriveMasterKey round-trip
func TestProperty_DeriveMasterKey(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random 32-byte key and encode as base64
		key := make([]byte, 32)
		rand.Read(key)
		b64 := base64.StdEncoding.EncodeToString(key)

		derived, err := DeriveMasterKey(b64)
		if err != nil {
			t.Fatalf("DeriveMasterKey failed: %v", err)
		}

		if len(derived) != 32 {
			t.Fatalf("derived key length: got %d, want 32", len(derived))
		}
		for i := range key {
			if derived[i] != key[i] {
				t.Fatalf("key mismatch at index %d", i)
			}
		}
	})
}
