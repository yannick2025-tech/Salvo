package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

// gcmNonceSize 是业务系统约定的 GCM nonce 长度（16 字节，非标准 12 字节）。
// 与业务后端（login.py 等）保持一致，不可修改。
const gcmNonceSize = 16

// GCMEncrypt 使用 AES-GCM 加密（业务格式）。
//
// 业务约定：nonce 长度 16 字节，由调用方传入；输出为 base64(ciphertext + tag)，
// tag 拼在密文末尾（GCM 默认行为），不包含 nonce。
//
// 参数：
//   - plaintext: 明文字符串
//   - key:       原始密钥字节（调用方负责 base64 解码）
//   - iv:        nonce 字节（长度必须为 16）
//
// 返回：base64 编码的 (ciphertext + tag)
func GCMEncrypt(plaintext string, key, iv []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, gcmNonceSize)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	ciphertext := gcm.Seal(nil, iv, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// GCMDecrypt 使用 AES-GCM 解密（业务格式，与 GCMEncrypt 对应）。
//
// 参数：
//   - ciphertextB64: base64 编码的 (ciphertext + tag)
//   - key:            原始密钥字节
//   - iv:             nonce 字节（长度必须为 16）
//
// 返回：明文字符串
func GCMDecrypt(ciphertextB64 string, key, iv []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, gcmNonceSize)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm decrypt: %w", err)
	}
	return string(plaintext), nil
}
