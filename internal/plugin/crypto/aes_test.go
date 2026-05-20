package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESKeyLengthValidation(t *testing.T) {
	_, err := NewAES([]byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "16, 24, or 32 bytes")
}

func TestAES128Key(t *testing.T) {
	key := make([]byte, 16)
	a, err := NewAES(key)
	require.NoError(t, err)
	assert.Equal(t, 128, a.KeyBits())
	assert.Equal(t, "aes-128-gcm", a.Algorithm())
}

func TestAES192Key(t *testing.T) {
	key := make([]byte, 24)
	a, err := NewAES(key)
	require.NoError(t, err)
	assert.Equal(t, 192, a.KeyBits())
	assert.Equal(t, "aes-192-gcm", a.Algorithm())
}

func TestAES256Key(t *testing.T) {
	a, err := NewAES(testKey())
	require.NoError(t, err)
	assert.Equal(t, 256, a.KeyBits())
	assert.Equal(t, "aes-256-gcm", a.Algorithm())
}

func TestAESModeName(t *testing.T) {
	a, err := NewAES(testKey(), WithMode(ModeCBC))
	require.NoError(t, err)
	assert.Equal(t, "cbc", a.ModeName())
}

func TestAESUnsupportedMode(t *testing.T) {
	_, err := NewAES(testKey(), WithMode(Mode(99)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestModeString(t *testing.T) {
	assert.Equal(t, "gcm", ModeGCM.String())
	assert.Equal(t, "cbc", ModeCBC.String())
	assert.Equal(t, "ctr", ModeCTR.String())
	assert.Contains(t, Mode(99).String(), "unknown")
}

func TestAESImplementsInterfaces(t *testing.T) {
	a, err := NewAES(testKey())
	require.NoError(t, err)
	var _ Encryptor = a
	var _ Decryptor = a
}
