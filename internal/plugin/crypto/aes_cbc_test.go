package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESCBCBasicRoundTrip(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)

	plaintext := []byte("hello CBC mode")
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESCBCAlgorithm(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)
	assert.Equal(t, "aes-256-cbc", a.Algorithm())
}

func TestAESCBCLargeData(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)

	plaintext := make([]byte, 1024)
	rand.Read(plaintext)

	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESCBCEmptyPlaintext(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)

	ciphertext, err := a.Encrypt([]byte{})
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestAESCBCDecryptTooShort(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)
	_, err = a.Decrypt([]byte("short"))
	assert.Error(t, err)
}

func TestAESCBCDecryptNotAligned(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)

	iv := make([]byte, 16)
	rand.Read(iv)
	badCiphertext := append(iv, []byte("not_aligned")...)

	_, err = a.Decrypt(badCiphertext)
	assert.Error(t, err)
}

func TestAESCBCWrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	rand.Read(key2)

	a1, err := NewAES(key1, WithMode(ModeCBC))
	require.NoError(t, err)
	a2, err := NewAES(key2, WithMode(ModeCBC))
	require.NoError(t, err)

	ciphertext, err := a1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	_, err = a2.Decrypt(ciphertext)
	assert.Error(t, err)
}

func TestAESCBCExactBlockSize(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)

	plaintext := make([]byte, 16)
	copy(plaintext, "exactly 16 bytes")

	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
