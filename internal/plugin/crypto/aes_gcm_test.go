package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCMBasicRoundTrip(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeGCM))
	require.NoError(t, err)

	plaintext := []byte("the quick brown fox jumps over the lazy dog")
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESGCMAlgorithm(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeGCM))
	require.NoError(t, err)
	assert.Equal(t, "aes-256-gcm", a.Algorithm())
}

func TestAESGCMDifferentCiphertexts(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeGCM))
	require.NoError(t, err)

	plaintext := []byte("same input")
	c1, err := a.Encrypt(plaintext)
	require.NoError(t, err)
	c2, err := a.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, c1, c2)
}

func TestAESGCMDecryptTooShort(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeGCM))
	require.NoError(t, err)
	_, err = a.Decrypt([]byte("short"))
	assert.Error(t, err)
}

func TestAESGCMWrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	rand.Read(key2)

	a1, err := NewAES(key1, WithMode(ModeGCM))
	require.NoError(t, err)
	a2, err := NewAES(key2, WithMode(ModeGCM))
	require.NoError(t, err)

	ciphertext, err := a1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	_, err = a2.Decrypt(ciphertext)
	assert.Error(t, err)
}

func TestAESGCMEmptyPlaintext(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeGCM))
	require.NoError(t, err)

	ciphertext, err := a.Encrypt([]byte{})
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestAESGCM128RoundTrip(t *testing.T) {
	key := make([]byte, 16)
	rand.Read(key)

	a, err := NewAES(key, WithMode(ModeGCM))
	require.NoError(t, err)

	plaintext := []byte("AES-128-GCM test")
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
