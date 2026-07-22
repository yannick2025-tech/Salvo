package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cbceEncryptWithIV is a test helper that encrypts plaintext using AES-CBC
// with an externally-provided IV (no prepend), matching the Manhattan convention.
func cbceEncryptWithIV(plaintext string, key, iv []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func TestCBCDecryptWithIV_RoundTrip(t *testing.T) {
	// 32-byte key (AES-256) + IV from key[:16] (Manhattan convention)
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	iv := key[:16]

	plaintext := `{"errorCode":0,"data":{"orderId":"202607211619060001"}}`
	encResult, err := cbceEncryptWithIV(plaintext, key, iv)
	require.NoError(t, err)

	decResult, err := CBCDecryptWithIV(encResult, key, iv)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

func TestCBCDecryptWithIV_ManhattanKey(t *testing.T) {
	// Simulate the Manhattan AES key format (base64-encoded 32-byte key)
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	// Decode and derive IV (Manhattan convention: IV = key[:16])
	key, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	require.NoError(t, err)
	iv := key[:16]

	plaintext := `{"errorCode":0,"data":{"orderId":"202607211619060001","status":"charging"}}`
	encResult, err := cbceEncryptWithIV(plaintext, key, iv)
	require.NoError(t, err)

	decResult, err := CBCDecryptWithIV(encResult, key, iv)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

func TestCBCDecryptWithIV_EmptyCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := key[:16]

	_, err := CBCDecryptWithIV("", key, iv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestCBCDecryptWithIV_InvalidBase64(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := key[:16]

	_, err := CBCDecryptWithIV("not-valid-base64!!", key, iv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode ciphertext")
}

func TestCBCDecryptWithIV_NotAligned(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	iv := key[:16]

	// Encode a 5-byte ciphertext (not aligned to 16-byte block size)
	invalidCiphertext := base64.StdEncoding.EncodeToString([]byte("hello"))

	_, err := CBCDecryptWithIV(invalidCiphertext, key, iv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not aligned")
}
