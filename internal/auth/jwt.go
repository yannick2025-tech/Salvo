// Package auth provides JWT-based authentication and RBAC authorization
// for Salvo. It handles token issuance, validation, role-based access
// control, and initial seed data for built-in roles and permissions.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

type Claims struct {
<<<<<<< HEAD
	UserID snowflake.ID `json:"uid"`
	RoleID snowflake.ID `json:"rid"`
=======
	UserID   snowflake.ID `json:"uid"`
	RoleName string       `json:"rn"`
>>>>>>> dev
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

<<<<<<< HEAD
func (m *JWTManager) Generate(userID, roleID snowflake.ID) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		RoleID: roleID,
=======
func (m *JWTManager) Generate(userID snowflake.ID, roleName string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   userID,
		RoleName: roleName,
>>>>>>> dev
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Issuer:    "salvo",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
