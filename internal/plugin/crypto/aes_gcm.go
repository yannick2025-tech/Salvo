package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"io"
)

// gcmMode implements aesMode using AES-GCM (AEAD).
type gcmMode struct{}

func (g *gcmMode) encrypt(block cipher.Block, plaintext []byte) ([]byte, error) {
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (g *gcmMode) decrypt(block cipher.Block, ciphertext []byte) ([]byte, error) {
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, nil)
}
