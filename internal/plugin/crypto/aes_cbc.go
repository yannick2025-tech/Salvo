package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"io"
)

// cbcMode implements aesMode using AES-CBC with PKCS7 padding.
// NOTE: CBC does NOT provide authentication. Use with HasherPlugin
// for integrity protection (encrypt-then-MAC pattern).
type cbcMode struct{}

func (c *cbcMode) encrypt(block cipher.Block, plaintext []byte) ([]byte, error) {
	padded := pkcs7Pad(plaintext, block.BlockSize())

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)
	return result, nil
}

func (c *cbcMode) decrypt(block cipher.Block, ciphertext []byte) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, errCiphertextTooShort
	}
	if len(ciphertext)%blockSize != 0 {
		return nil, errCiphertextNotAligned
	}

	iv := ciphertext[:blockSize]
	ciphertext = ciphertext[blockSize:]

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpad(plaintext, blockSize)
}
