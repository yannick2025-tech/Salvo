// Package main implements the shell-aes SO plugin for AES-CBC encryption/decryption.
//
// Build: go build -buildmode=plugin -o shell-aes.so plugins/shell-aes/main.go
//
// The plugin matches the AES-CBC cipher used by login.py (AESCiphers):
//   - Key: raw string (e.g. 32-byte "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
//   - IV: base64-encoded (e.g. "BBBBBBBBBBBBBBBBBBBBBA==" decodes to 16 bytes)
//   - Encrypt output: base64-encoded ciphertext
//   - Decrypt input: base64-encoded ciphertext
//
// Usage from expressions:
//   ${__so("shell-aes", "encrypt", "key", "iv", "plaintext")}
//   ${__so("shell-aes", "decrypt", "key", "iv", "base64ciphertext")}
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
)

// shellAES implements the so.Plugin interface for AES-CBC encryption.
type shellAES struct{}

// Name returns the plugin name.
func (s *shellAES) Name() string { return "shell-aes" }

// Version returns the plugin version.
func (s *shellAES) Version() string { return "1.0.0" }

// Call executes the named operation.
// Supported operations:
//   - encrypt(key, iv, plaintext) → base64 ciphertext
//   - decrypt(key, iv, ciphertext) → plaintext
func (s *shellAES) Call(op string, args []string) (string, error) {
	switch op {
	case "encrypt":
		return s.encrypt(args)
	case "decrypt":
		return s.decrypt(args)
	default:
		return "", fmt.Errorf("unknown operation %q", op)
	}
}

// encrypt performs AES-CBC encryption.
// Args: [key, iv_base64, plaintext]
// Returns: base64-encoded ciphertext (IV prepended, matching login.py format).
func (s *shellAES) encrypt(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("encrypt requires 3 args: key, iv (base64), plaintext")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decoding iv: %w", err)
	}
	plaintext := []byte(args[2])

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	// PKCS7 padding.
	padded := pkcs7Pad(plaintext, aes.BlockSize)

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV (matching login.py AESCiphers format).
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)

	return base64.StdEncoding.EncodeToString(result), nil
}

// decrypt performs AES-CBC decryption.
// Args: [key, iv_base64, ciphertext_base64]
// Returns: plaintext string.
func (s *shellAES) decrypt(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("decrypt requires 3 args: key, iv (base64), ciphertext (base64)")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decoding iv: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(args[2])
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	// login.py format: IV is prepended to ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	iv = ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext not aligned to block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("unpad: %w", err)
	}

	return string(plaintext), nil
}

// pkcs7Pad adds PKCS7 padding to the given data.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

// pkcs7Unpad removes PKCS7 padding from the given data.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("unpad: empty data")
	}
	if len(data)%blockSize != 0 {
		return nil, errors.New("unpad: data not aligned")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("unpad: invalid padding length")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, errors.New("unpad: invalid padding byte")
		}
	}
	return data[:len(data)-padding], nil
}

// New is the factory function exported for the SO plugin loader.
func New() (so.Plugin, error) {
	return &shellAES{}, nil
}