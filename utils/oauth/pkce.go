package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds a PKCE verifier and challenge pair.
type PKCE struct {
	Verifier  string `json:"verifier"`
	Challenge string `json:"challenge"`
}

// GeneratePKCE generates a PKCE code verifier and challenge.
func GeneratePKCE() (PKCE, error) {
	// Generate random verifier (43-128 characters)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate challenge (S256)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	return PKCE{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
