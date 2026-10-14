// Package main implements the aes SO plugin for AES encryption/decryption.
//
// Build: go build -buildmode=plugin -o aes.so plugins/aes/main.go
//
// 支持两种模式：
//   - AES-CBC (兼容 login.py AESCiphers 格式)：IV 前置到密文，PKCS7 padding
//   - AES-GCM (业务格式)：16 字节 nonce，外部传入 IV，输出 base64(ciphertext+tag)
//
// Usage from expressions:
//
//	${__so("aes", "encrypt", "key", "iv", "plaintext")}        # CBC
//	${__so("aes", "decrypt", "key", "iv", "base64ciphertext")} # CBC
//	${__so("aes", "encrypt_gcm", "key", "iv", "plaintext")}    # GCM
//	${__so("aes", "decrypt_gcm", "key", "iv", "base64ciphertext")} # GCM
package main

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/plugin/so/contract"
	"github.com/yannick2025-tech/Salvo/plugins/shared/crypto"
)

// aesPlugin implements the so.Plugin interface for AES encryption.
type aesPlugin struct{}

// Name returns the plugin name.
func (s *aesPlugin) Name() string { return "aes" }

// Version returns the plugin version.
func (s *aesPlugin) Version() string { return "1.0.0" }

// Call executes the named operation.
// Supported operations:
//   - encrypt(key, iv_base64, plaintext) → base64 ciphertext (CBC, IV prepended)
//   - decrypt(key, iv_base64, ciphertext_base64) → plaintext (CBC, IV read from ciphertext head)
//   - encrypt_gcm(key, iv_base64, plaintext) → base64 ciphertext (GCM)
//   - decrypt_gcm(key, iv_base64, ciphertext_base64) → plaintext (GCM)
func (s *aesPlugin) Call(op string, args []string) (string, error) {
	switch op {
	case "encrypt":
		return s.encryptCBC(args)
	case "decrypt":
		return s.decryptCBC(args)
	case "encrypt_gcm":
		return s.encryptGCM(args)
	case "decrypt_gcm":
		return s.decryptGCM(args)
	default:
		return "", fmt.Errorf("unknown operation %q", op)
	}
}

// encryptCBC performs AES-CBC encryption.
// Args: [key, iv_base64, plaintext]
// Returns: base64-encoded ciphertext (IV prepended, matching login.py format).
func (s *aesPlugin) encryptCBC(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("encrypt requires 3 args: key, iv (base64), plaintext")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decoding iv: %w", err)
	}
	return crypto.CBCEncrypt(args[2], key, iv)
}

// decryptCBC performs AES-CBC decryption.
// Args: [key, iv_base64, ciphertext_base64]
// Note: IV is read from the ciphertext head (login.py format); the iv_base64
// argument is accepted for API compatibility but ignored.
// Returns: plaintext string.
func (s *aesPlugin) decryptCBC(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("decrypt requires 3 args: key, iv (base64), ciphertext (base64)")
	}
	key := []byte(args[0])
	// iv_base64 (args[1]) is ignored: IV is prepended in the ciphertext.
	return crypto.CBCDecrypt(args[2], key)
}

// encryptGCM performs AES-GCM encryption (business format: 16-byte nonce).
// Args: [key, iv_base64, plaintext]
// Returns: base64-encoded ciphertext + tag.
func (s *aesPlugin) encryptGCM(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("encrypt_gcm requires 3 args: key, iv (base64), plaintext")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decoding iv: %w", err)
	}
	return crypto.GCMEncrypt(args[2], key, iv)
}

// decryptGCM performs AES-GCM decryption (business format).
// Args: [key, iv_base64, ciphertext_base64]
// Returns: plaintext string.
func (s *aesPlugin) decryptGCM(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("decrypt_gcm requires 3 args: key, iv (base64), ciphertext (base64)")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decoding iv: %w", err)
	}
	return crypto.GCMDecrypt(args[2], key, iv)
}

// New is the factory function exported for the SO plugin loader.
func New() (contract.Plugin, error) {
	return &aesPlugin{}, nil
}
