// Package crypto provides AES-256-GCM encryption, bcrypt hashing, and JWT operations.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// DeriveMasterKey decodes a base64-encoded string into a 32-byte AES key.
func DeriveMasterKey(b64Key string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		// Try URL-safe base64
		key, err = base64.URLEncoding.DecodeString(b64Key)
		if err != nil {
			// Try raw (no padding) variants
			key, err = base64.RawStdEncoding.DecodeString(b64Key)
			if err != nil {
				return nil, fmt.Errorf("crypto: invalid base64 master key: %w", err)
			}
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: master key must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Output format: base64(nonce[12] + ciphertext + tag[16])
func Encrypt(key []byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
// Input format: base64(nonce[12] + ciphertext + tag[16])
func Decrypt(key []byte, cipherB64 string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt failed: %w", err)
	}

	return plaintext, nil
}
