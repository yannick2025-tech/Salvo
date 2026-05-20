package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMACSHA256Algorithm(t *testing.T) {
	h := NewHMACSHA256(testKey())
	assert.Equal(t, "hmac-sha256", h.Algorithm())
}

func TestHMACSHA256Hash(t *testing.T) {
	key := testKey()
	h := NewHMACSHA256(key)

	data := []byte("hello world")
	result := h.Hash(data)

	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, result)
}

func TestHMACSHA256HashDeterministic(t *testing.T) {
	h := NewHMACSHA256(testKey())
	data := []byte("same input")
	r1 := h.Hash(data)
	r2 := h.Hash(data)
	assert.Equal(t, r1, r2)
}

func TestHMACSHA256HashDifferentInputs(t *testing.T) {
	h := NewHMACSHA256(testKey())
	r1 := h.Hash([]byte("input1"))
	r2 := h.Hash([]byte("input2"))
	assert.NotEqual(t, r1, r2)
}

func TestHMACSHA256Verify(t *testing.T) {
	h := NewHMACSHA256(testKey())
	data := []byte("test data")
	digest := h.Hash(data)
	assert.True(t, h.Verify(data, digest))
}

func TestHMACSHA256VerifyMismatch(t *testing.T) {
	h := NewHMACSHA256(testKey())
	assert.False(t, h.Verify([]byte("data"), "invalidhex"))
}

func TestHMACSHA256VerifyInvalidHex(t *testing.T) {
	h := NewHMACSHA256(testKey())
	assert.False(t, h.Verify([]byte("data"), "not-hex-at-all"))
}

func TestHMACSHA256EmptyData(t *testing.T) {
	h := NewHMACSHA256(testKey())
	digest := h.Hash([]byte{})
	assert.NotEmpty(t, digest)
	assert.True(t, h.Verify([]byte{}, digest))
}
