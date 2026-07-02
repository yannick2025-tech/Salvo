// Package main - paypwd SO plugin for Salvo
//
// 功能：支付密码生成插件
// 编译：go build -buildmode=plugin -o paypwd.so main.go
//
// 调用语法：${__so("paypwd","gen", baseurl, token, aesKey, userId, password)}
// 返回：加密后的支付密码字符串（base64 编码）
//
// 内部流程（与 snailx GenPayPwd 完全一致）：
//  1. 调用 getPayAppSalt API 获取 salt
//  2. bcrypt hash: bcrypt(password, salt)
//  3. 构造明文: aesKey + ":" + bcryptHash
//  4. AES-GCM 加密(使用 aesKey 的前 16 字节作为 IV)
//  5. 返回 base64 编码的密文
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/plugins/shared/crypto"
)

// Run 是 Salvo SO 插件的标准入口
func Run(operation string, args ...string) (string, error) {
	switch operation {
	case "gen":
		return genPayPwd(args...)
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

// genPayPwd 生成支付密码
// args[0]: baseurl - 基础 URL (如 https://uat-manhattan.shell.com.cn)
// args[1]: token - C端用户 token
// args[2]: aesKey - AES 密钥 (base64 编码)
// args[3]: userId - 用户 ID (字符串)
// args[4]: password - 明文密码 (默认 "123456")
func genPayPwd(args ...string) (string, error) {
	if len(args) < 5 {
		return "", fmt.Errorf("gen requires 5 args: baseurl, token, aesKey, userId, password")
	}
	baseurl := args[0]
	token := args[1]
	aesKey := args[2]
	userId := args[3]
	password := args[4]

	// Step 1: 获取 pay app salt
	salt, err := getPayAppSalt(baseurl, token, aesKey, userId)
	if err != nil {
		return "", fmt.Errorf("get pay app salt failed: %v", err)
	}

	// Step 2: bcrypt hash
	bcryptHash, err := passwordEncode(password, salt)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash failed: %v", err)
	}

	// Step 3: 构造明文 aesKey:bcryptHash
	plaintext := aesKey + ":" + bcryptHash

	// Step 4: AES-GCM 加密
	iv, err := getIV(aesKey)
	if err != nil {
		return "", fmt.Errorf("get IV failed: %v", err)
	}

	encrypted, err := base64AESGCMEncrypt(plaintext, aesKey, iv)
	if err != nil {
		return "", fmt.Errorf("AES-GCM encrypt failed: %v", err)
	}

	return encrypted, nil
}

// --- HTTP 调用获取 salt ---

type getPayAppSaltReq struct {
	UserID interface{} `json:"userId"`
}

type getPayAppSaltRes struct {
	Data      *getPayAppSaltData `json:"data,omitempty"`
	ErrorCode int64              `json:"errorCode,omitempty"`
	ErrorMsg  *string            `json:"errorMsg,omitempty"`
}

type getPayAppSaltData struct {
	PayAppSalt *string `json:"payAppSalt,omitempty"`
}

func getPayAppSalt(baseurl, token, aesKey, userId string) (string, error) {
	url := baseurl + "/ev/crm/crm-service/v1/wx/get-pay-app-salt"

	reqBody := map[string]interface{}{
		"userId": userId,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 尝试解密 AES 响应（如果响应被加密了）
	respStr := string(respBody)
	if len(respStr) > 0 && respStr[0] != '{' {
		// 响应可能是 AES 加密的，尝试解密
		decrypted, decErr := aesDecryptResponse(respStr, aesKey)
		if decErr == nil {
			respStr = decrypted
		}
	}

	var result getPayAppSaltRes
	if err := json.Unmarshal([]byte(respStr), &result); err != nil {
		return "", fmt.Errorf("parse response failed: %v, body: %s", err, respStr)
	}

	if result.ErrorCode != 0 {
		errMsg := "unknown error"
		if result.ErrorMsg != nil {
			errMsg = *result.ErrorMsg
		}
		return "", fmt.Errorf("API error: %s", errMsg)
	}

	if result.Data == nil || result.Data.PayAppSalt == nil {
		return "", fmt.Errorf("payAppSalt is empty in response")
	}

	return *result.Data.PayAppSalt, nil
}

// --- 加密工具函数（与 snailx ciphers 包完全一致）---

// passwordEncode 对应 ciphers.PasswordEncode
func passwordEncode(pwd, salt string) (string, error) {
	if strings.Contains(salt, "$") {
		s := strings.Split(salt, "$")
		salt = s[len(s)-1]
	}
	bytes, err := crypto.GenerateFromPassword([]byte(pwd), salt, crypto.BcryptDefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// getIV 对应 base.GetIV：取 aesKey base64 解码后的前 16 字节作为 IV
func getIV(aesKey string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return "", err
	}
	s := b[:16]
	return base64.StdEncoding.EncodeToString(s), nil
}

// base64AESGCMEncrypt 对应 ciphers.Base64AESGCMEncrypt
// key/iv 均为 base64 编码字符串，内部解码后调用公共 GCM 加密。
func base64AESGCMEncrypt(p, key, iv string) (string, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	i, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}
	return crypto.GCMEncrypt(p, k, i)
}

// aesDecryptResponse 尝试 AES-GCM 解密响应体
// aesKey 为 base64 编码，IV 取 key 解码后的前 16 字节。
func aesDecryptResponse(ciphertext, aesKey string) (string, error) {
	k, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return "", fmt.Errorf("decode aesKey: %w", err)
	}
	iv := k[:16]
	return crypto.GCMDecrypt(ciphertext, k, iv)
}
