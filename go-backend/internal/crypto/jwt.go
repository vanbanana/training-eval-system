package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims represents the JWT payload.
type Claims struct {
	Sub      int64  `json:"sub"` // user ID
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"` // "access" or "refresh"
	Exp      int64  `json:"exp"`  // expiration (unix timestamp)
	Iat      int64  `json:"iat"`  // issued at (unix timestamp)
}

// IsExpired returns true if the token has expired.
func (c *Claims) IsExpired() bool {
	return time.Now().Unix() > c.Exp
}

// SignToken creates a JWT (HS256) from the given claims.
func SignToken(secret string, claims *Claims) (string, error) {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("crypto: marshal claims: %w", err)
	}
	payloadB64 := base64URLEncode(payload)

	signingInput := header + "." + payloadB64
	signature := hmacSHA256([]byte(secret), []byte(signingInput))
	sigB64 := base64URLEncode(signature)

	return signingInput + "." + sigB64, nil
}

// VerifyToken parses and verifies a JWT (HS256), returning the claims.
func VerifyToken(secret string, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("crypto: invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := hmacSHA256([]byte(secret), []byte(signingInput))
	actualSig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("crypto: decode signature: %w", err)
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, fmt.Errorf("crypto: invalid signature")
	}

	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("crypto: decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("crypto: unmarshal claims: %w", err)
	}

	if claims.IsExpired() {
		return nil, fmt.Errorf("crypto: token expired")
	}

	return &claims, nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
