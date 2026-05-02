package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// ctrMode implements aesMode using AES-CTR.
// NOTE: CTR does NOT provide authentication. Use with HasherPlugin
// for integrity protection (encrypt-then-MAC pattern).
type ctrMode struct{}

func (c *ctrMode) encrypt(block cipher.Block, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewCTR(block, nonce)
	stream.XORKeyStream(ciphertext, plaintext)

	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)
	return result, nil
}

func (c *ctrMode) decrypt(block cipher.Block, ciphertext []byte) ([]byte, error) {
	nonceSize := block.BlockSize()
	if len(ciphertext) < nonceSize {
		return nil, errCiphertextTooShort
	}

	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, nonce)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// errCiphertextNotAligned is returned when CBC ciphertext is not
// a multiple of the block size.
var errCiphertextNotAligned = errors.New("ciphertext is not a multiple of the block size")

// pkcs7Pad pads data to a multiple of blockSize using PKCS7.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

// pkcs7Unpad removes PKCS7 padding.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	if len(data)%blockSize != 0 {
		return nil, errors.New("data is not a multiple of block size")
	}

	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("invalid padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}
