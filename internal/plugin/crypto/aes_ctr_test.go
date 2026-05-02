package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESCTRBasicRoundTrip(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)

	plaintext := []byte("hello CTR mode")
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESCTRAlgorithm(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)
	assert.Equal(t, "aes-256-ctr", a.Algorithm())
}

func TestAESCTRNoPadding(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)

	plaintext := []byte("any length data without padding")
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)

	nonceSize := 16
	assert.Equal(t, nonceSize+len(plaintext), len(ciphertext))
}

func TestAESCTREmptyPlaintext(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)

	ciphertext, err := a.Encrypt([]byte{})
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestAESCTRDecryptTooShort(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)
	_, err = a.Decrypt([]byte("short"))
	assert.Error(t, err)
}

func TestAESCTRWrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	rand.Read(key2)

	a1, err := NewAES(key1, WithMode(ModeCTR))
	require.NoError(t, err)
	a2, err := NewAES(key2, WithMode(ModeCTR))
	require.NoError(t, err)

	plaintext := []byte("secret message that should be garbled")
	ciphertext, err := a1.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := a2.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, decrypted, "CTR without auth: wrong key produces garbage, not an error")
}

func TestAESCTRStreamLike(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCTR))
	require.NoError(t, err)

	plaintext := []byte{0x01, 0x02, 0x03}
	ciphertext, err := a.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := a.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
