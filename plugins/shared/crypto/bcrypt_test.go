package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid bcrypt salt: 22 chars of bcrypt base64 alphabet (./A-Za-z0-9)
const testBcryptSalt = "0123456789012345678901"

func TestGenerateFromPassword_Valid(t *testing.T) {
	password := []byte("test-password-123")

	hash, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, string(hash), "$2a$")
}

func TestBcryptCompareHashAndPassword_Valid(t *testing.T) {
	password := []byte("test-password-123")

	hash, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)

	err = BcryptCompareHashAndPassword(hash, password)
	assert.NoError(t, err)
}

func TestBcryptCompareHashAndPassword_Invalid(t *testing.T) {
	password := []byte("test-password-123")
	wrongPassword := []byte("wrong-password")

	hash, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)

	err = BcryptCompareHashAndPassword(hash, wrongPassword)
	assert.Error(t, err)
}

func TestBcryptCompareHashAndPassword_InvalidHash(t *testing.T) {
	password := []byte("test-password-123")
	invalidHash := []byte("not-a-valid-bcrypt-hash")

	err := BcryptCompareHashAndPassword(invalidHash, password)
	assert.Error(t, err)
}

func TestGenerateFromPassword_DifferentSalts(t *testing.T) {
	password := []byte("test-password-123")
	salt1 := "0123456789012345678901"
	salt2 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	hash1, err := GenerateFromPassword(password, salt1, BcryptDefaultCost)
	require.NoError(t, err)

	hash2, err := GenerateFromPassword(password, salt2, BcryptDefaultCost)
	require.NoError(t, err)

	assert.NotEqual(t, string(hash1), string(hash2))
}

func TestGenerateFromPassword_EmptyPassword(t *testing.T) {
	password := []byte("")

	hash, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	err = BcryptCompareHashAndPassword(hash, password)
	assert.NoError(t, err)
}

func TestGenerateFromPassword_LongPassword(t *testing.T) {
	// Bcrypt has a 72-byte limit
	password := make([]byte, 73)
	for i := range password {
		password[i] = 'a'
	}

	_, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	assert.Error(t, err)
}

func TestGenerateFromPassword_FullSaltFormat(t *testing.T) {
	password := []byte("test-password")
	// Full bcrypt salt format: $2a$10$ + 22-char salt
	fullSalt := "$2a$10$0123456789012345678901"

	hash, err := GenerateFromPassword(password, fullSalt, BcryptDefaultCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify roundtrip
	err = BcryptCompareHashAndPassword(hash, password)
	assert.NoError(t, err)
}

func TestGenerateFromPassword_LowCost(t *testing.T) {
	password := []byte("test-password")

	// Cost below minimum should be adjusted to default
	hash, err := GenerateFromPassword(password, testBcryptSalt, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestGenerateFromPassword_Deterministic(t *testing.T) {
	password := []byte("deterministic-test")

	hash1, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)

	hash2, err := GenerateFromPassword(password, testBcryptSalt, BcryptDefaultCost)
	require.NoError(t, err)

	// Same password + same salt = same hash (deterministic)
	assert.Equal(t, string(hash1), string(hash2))
}
