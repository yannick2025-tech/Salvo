package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// AESGCM implements both Encryptor and Decryptor using AES-256-GCM.
type AESGCM struct {
	key []byte
}

// NewAESGCM creates an AES-GCM cipher. The key must be exactly 32
// bytes (AES-256).
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: AES-256 key must be 32 bytes")
	}
	a := &AESGCM{key: make([]byte, 32)}
	copy(a.key, key)
	return a, nil
}

// Algorithm implements Encryptor / Decryptor.
func (a *AESGCM) Algorithm() string { return "aes-256-gcm" }

// Encrypt implements Encryptor.
func (a *AESGCM) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt implements Decryptor.
func (a *AESGCM) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
