package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
)

// CBCEncrypt 使用 AES-CBC 加密（业务格式：IV 前置 + PKCS7 padding）。
//
// 业务约定（兼容 login.py AESCiphers）：
//   - PKCS7 padding
//   - IV 前置到密文：result = iv + ciphertext
//   - 整体 base64 编码输出
//
// 参数：
//   - plaintext: 明文字符串
//   - key:       原始密钥字节
//   - iv:        IV 字节（长度必须等于 aes.BlockSize）
//
// 返回：base64 编码的 (iv + ciphertext)
func CBCEncrypt(plaintext string, key, iv []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV (matching login.py AESCiphers format).
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)
	return base64.StdEncoding.EncodeToString(result), nil
}

// CBCDecrypt 使用 AES-CBC 解密（业务格式，与 CBCEncrypt 对应）。
//
// 业务约定：IV 从密文头部读取（前 aes.BlockSize 字节），剩余为密文。
//
// 参数：
//   - ciphertextB64: base64 编码的 (iv + ciphertext)
//   - key:          原始密钥字节
//
// 返回：明文字符串
func CBCDecrypt(ciphertextB64 string, key []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	if len(raw) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	iv, ciphertext := raw[:aes.BlockSize], raw[aes.BlockSize:]
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

// pkcs7Pad adds PKCS7 padding.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

// pkcs7Unpad removes PKCS7 padding.
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
