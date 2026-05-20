package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACSHA256 implements both Hasher and Verifier using HMAC-SHA256.
type HMACSHA256 struct {
	key []byte
}

// NewHMACSHA256 creates an HMAC-SHA256 signer/verifier.
func NewHMACSHA256(key []byte) *HMACSHA256 {
	h := &HMACSHA256{key: make([]byte, len(key))}
	copy(h.key, key)
	return h
}

// Algorithm implements Hasher / Verifier.
func (h *HMACSHA256) Algorithm() string { return "hmac-sha256" }

// Hash implements Hasher.
func (h *HMACSHA256) Hash(data []byte) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify implements Verifier.
func (h *HMACSHA256) Verify(data []byte, expectedHex string) bool {
	computed := h.Hash(data)
	computedBytes, err := hex.DecodeString(computed)
	if err != nil {
		return false
	}
	expectedBytes, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}
	return hmac.Equal(computedBytes, expectedBytes)
}
