package runner

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manhattanEncrypt simulates Manhattan API AES-CBC encryption:
// - key: base64-encoded 32-byte key
// - IV: key[:16]
// - output: JSON string (with quotes) containing base64 ciphertext
func manhattanEncrypt(plaintext string, aesKeyBase64 string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	if err != nil {
		return nil, err
	}
	iv := keyBytes[:16]

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	// PKCS7 pad
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// The API returns the ciphertext as a JSON string (with quotes)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return json.Marshal(encoded)
}

func TestAesDecryptResponse_CBC(t *testing.T) {
	// Simulate a Manhattan AES key
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	plaintext := `{"errorCode":0,"data":{"orderId":"202607211619060001"}}`

	// Encrypt using Manhattan convention
	encryptedBody, err := manhattanEncrypt(plaintext, aesKeyBase64)
	require.NoError(t, err)

	// Verify the encrypted body doesn't start with '{' (it's a quoted string)
	assert.NotEqual(t, uint8('{'), encryptedBody[0])

	// Decrypt
	decrypted, err := aesDecryptResponse(encryptedBody, aesKeyBase64, 0)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAesDecryptResponse_PlainJSON(t *testing.T) {
	// When the response is plain JSON, the caller should check before calling
	// aesDecryptResponse, but even if called, it should handle gracefully
	aesKeyBase64 := base64.StdEncoding.EncodeToString(make([]byte, 32))

	plainBody := []byte(`{"errorCode":0,"data":{}}`)

	// This would fail because the body starts with '{' and json.Unmarshal
	// would give a map, not a string. But the caller checks body[0] != '{'
	// before calling, so this path isn't normally reached.
	// However, we still test that if someone passes a non-encrypted body,
	// the function returns an error (not a panic).
	_, err := aesDecryptResponse(plainBody, aesKeyBase64, 0)
	// Should error because the body is a JSON object, not a string
	assert.Error(t, err)
}

func TestAesDecryptResponse_InvalidKey(t *testing.T) {
	encryptedBody := []byte(`"dGVzdA=="`) // base64 of "test"

	_, err := aesDecryptResponse(encryptedBody, "not-valid-base64!!", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode aes key")
}

func TestAesDecryptResponse_EmptyBody(t *testing.T) {
	aesKeyBase64 := base64.StdEncoding.EncodeToString(make([]byte, 32))

	// Empty ciphertext after JSON unmarshal
	emptyBody := []byte(`""`)

	_, err := aesDecryptResponse(emptyBody, aesKeyBase64, 0)
	assert.Error(t, err)
}

func TestAesDecryptResponse_CBC_LargePayload(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	// Large JSON payload
	largePayload := `{"errorCode":0,"data":{"items":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			largePayload += ","
		}
		largePayload += `{"id":` + string(rune('0'+i%10)) + `}`
	}
	largePayload += `]}}`

	encryptedBody, err := manhattanEncrypt(largePayload, aesKeyBase64)
	require.NoError(t, err)

	decrypted, err := aesDecryptResponse(encryptedBody, aesKeyBase64, 0)
	require.NoError(t, err)
	assert.Equal(t, largePayload, decrypted)
}
