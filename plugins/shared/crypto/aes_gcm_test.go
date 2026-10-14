package crypto

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCMEncrypt_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes for AES-256
	iv := []byte("0123456789abcdef")                  // 16 bytes nonce
	plaintext := `{"errorCode":0,"data":{"orderId":"202607211619060001"}}`

	encrypted, err := GCMEncrypt(plaintext, key, iv)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := GCMDecrypt(encrypted, key, iv)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestGCMEncrypt_DifferentIV(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv1 := []byte("0123456789abcdef")
	iv2 := []byte("abcdef0123456789")
	plaintext := "test plaintext"

	// Encrypt with different IVs - should produce different ciphertexts
	enc1, err := GCMEncrypt(plaintext, key, iv1)
	require.NoError(t, err)

	enc2, err := GCMEncrypt(plaintext, key, iv2)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "Same plaintext with different IVs should produce different ciphertexts")
}

func TestGCMEncrypt_InvalidKey(t *testing.T) {
	invalidKey := []byte("short") // Invalid key length
	iv := []byte("0123456789abcdef")
	plaintext := "test"

	_, err := GCMEncrypt(plaintext, invalidKey, iv)
	assert.Error(t, err)
}

func TestGCMDecrypt_InvalidCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := []byte("0123456789abcdef")

	_, err := GCMDecrypt("invalid-base64!!", key, iv)
	assert.Error(t, err)
}

func TestGCMDecrypt_WrongKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("abcdef0123456789abcdef0123456789")
	iv := []byte("0123456789abcdef")
	plaintext := "test plaintext"

	encrypted, err := GCMEncrypt(plaintext, key1, iv)
	require.NoError(t, err)

	_, err = GCMDecrypt(encrypted, key2, iv)
	assert.Error(t, err)
}

func TestGCMDecrypt_TamperedCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := []byte("0123456789abcdef")
	plaintext := "test plaintext"

	encrypted, err := GCMEncrypt(plaintext, key, iv)
	require.NoError(t, err)

	// Decode and tamper with ciphertext
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)

	// Tamper with the ciphertext
	if len(ciphertext) > 0 {
		ciphertext[len(ciphertext)-1] ^= 0xff
	}
	tampered := base64.StdEncoding.EncodeToString(ciphertext)

	_, err = GCMDecrypt(tampered, key, iv)
	assert.Error(t, err)
}

func TestGCMEncrypt_EmptyPlaintext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := []byte("0123456789abcdef")

	encrypted, err := GCMEncrypt("", key, iv)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := GCMDecrypt(encrypted, key, iv)
	require.NoError(t, err)
	assert.Equal(t, "", decrypted)
}
