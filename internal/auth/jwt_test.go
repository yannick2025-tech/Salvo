package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret"
	ttl := 24 * time.Hour

	mgr := NewJWTManager(secret, ttl)
	require.NotNil(t, mgr)
	assert.Equal(t, []byte(secret), mgr.secret)
	assert.Equal(t, ttl, mgr.ttl)
}

func TestJWTManager_Generate(t *testing.T) {
	secret := "test-secret"
	ttl := 24 * time.Hour
	mgr := NewJWTManager(secret, ttl)

	userID := snowflake.ID(123456789)
	roleName := "admin"

	token, err := mgr.Generate(userID, roleName)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestJWTManager_Parse_Valid(t *testing.T) {
	secret := "test-secret"
	ttl := 24 * time.Hour
	mgr := NewJWTManager(secret, ttl)

	userID := snowflake.ID(123456789)
	roleName := "admin"

	token, err := mgr.Generate(userID, roleName)
	require.NoError(t, err)

	claims, err := mgr.Parse(token)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, roleName, claims.RoleName)
	assert.Equal(t, "salvo", claims.Issuer)
}

func TestJWTManager_Parse_InvalidToken(t *testing.T) {
	secret := "test-secret"
	ttl := 24 * time.Hour
	mgr := NewJWTManager(secret, ttl)

	claims, err := mgr.Parse("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTManager_Parse_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	ttl := -1 * time.Hour // Negative TTL = already expired
	mgr := NewJWTManager(secret, ttl)

	userID := snowflake.ID(123456789)
	roleName := "admin"

	token, err := mgr.Generate(userID, roleName)
	require.NoError(t, err)

	claims, err := mgr.Parse(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTManager_Parse_WrongSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret1", 24*time.Hour)
	mgr2 := NewJWTManager("secret2", 24*time.Hour)

	userID := snowflake.ID(123456789)
	roleName := "admin"

	token, err := mgr1.Generate(userID, roleName)
	require.NoError(t, err)

	claims, err := mgr2.Parse(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}
