// Package main implements the login SO plugin for B端登录流程.
//
// Build: go build -buildmode=plugin -o login.so plugins/login/main.go
//
// 登录流程 (login.md):
//
//  1. AES-GCM encrypt username with hardcoded SECRET_KEY/IV
//  2. POST {salt_base_url}/get-app-salt  →  secretKey, iv, saltStr(加密)
//  3. AES-GCM decrypt saltStr  →  "secretKey:$2a$10$salt"
//  4. bcrypt.hashpw(password, salt)  →  hashedPassword
//  5. AES-GCM encrypt "secretKey:hashedPassword"  →  loginInfo
//  6. POST {login_base_url}/username-login  →  jwtToken
//
// Usage:
//
//	${__so("login", "login", "salt_base_url", "login_base_url", "username", "password")}
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/plugin/so/contract"
	"github.com/yannick2025-tech/Salvo/plugins/login/bcryptsalt"
)

const (
	// 全局密钥，用于初始用户名加密（login.md 固定值，均为 Base64 编码）.
	// SECRET_KEY Base64 解码后为 24 字节原始密钥.
	defaultSecretKeyB64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	// IV Base64 解码后为 12 字节 nonce（GCM 标准长度）.
	defaultIVB64 = "BBBBBBBBBBBBBBBBBBBBBA=="
	httpTimeout  = 15 * time.Second
)

type loginPlugin struct {
	client *http.Client
}

func (p *loginPlugin) Name() string    { return "login" }
func (p *loginPlugin) Version() string { return "1.0.0" }

func (p *loginPlugin) Call(op string, args []string) (string, error) {
	switch op {
	case "login":
		return p.login(args)
	case "encrypt_username":
		return p.encryptUsername(args)
	case "get_salt":
		return p.getSalt(args)
	case "decrypt_salt":
		return p.decryptSalt(args)
	case "bcrypt_hash":
		return p.bcryptHash(args)
	case "build_login_info":
		return p.buildLoginInfo(args)
	case "username_login":
		return p.usernameLogin(args)
	default:
		return "", fmt.Errorf("unknown operation %q, supported: login, encrypt_username, get_salt, decrypt_salt, bcrypt_hash, build_login_info, username_login", op)
	}
}

// --- Operation: login (full flow) ---
//
// Args: [salt_base_url, login_base_url, username, password] → JWT token
func (p *loginPlugin) login(args []string) (string, error) {
	if len(args) < 4 {
		return "", errors.New("login requires 4 args: salt_base_url, login_base_url, username, password")
	}
	saltBaseURL := strings.TrimRight(args[0], "/")
	loginBaseURL := strings.TrimRight(args[1], "/")
	username := args[2]
	password := args[3]

	// Step 1: AES-GCM encrypt username with default key/IV (both Base64 encoded per login.md).
	defaultKeyRaw, err := base64.StdEncoding.DecodeString(defaultSecretKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode default secret key: %w", err)
	}
	encUsername, err := p.encryptUsername([]string{string(defaultKeyRaw), defaultIVB64, username})
	if err != nil {
		return "", fmt.Errorf("encrypt username: %w", err)
	}

	// Step 2: POST /get-app-salt.
	saltResp, err := p.getSalt([]string{saltBaseURL, encUsername})
	if err != nil {
		return "", fmt.Errorf("get app salt: %w", err)
	}
	var saltData struct {
		SecretKey string `json:"secretKey"`
		IV        string `json:"iv"`
		SaltStr   string `json:"saltStr"`
	}
	if err := json.Unmarshal([]byte(saltResp), &saltData); err != nil {
		return "", fmt.Errorf("parse salt response: %w", err)
	}

	// Step 3: AES-GCM decrypt saltStr → "secretKey:bCryptSalt".
	rawSalt, err := p.decryptSalt([]string{saltData.SecretKey, saltData.IV, saltData.SaltStr})
	if err != nil {
		return "", fmt.Errorf("decrypt salt: %w", err)
	}

	parts := strings.SplitN(rawSalt, ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid salt format: expected 'secretKey:salt', got %q", rawSalt)
	}
	bcryptSalt := parts[1]

	// Step 4: BCrypt hashpw(password, salt).
	hashedPwd, err := p.bcryptHash([]string{password, bcryptSalt})
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}

	// Step 5: AES-GCM encrypt "secretKey:hashedPassword" → loginInfo.
	encLoginInfo, err := p.buildLoginInfo([]string{saltData.SecretKey, saltData.IV, hashedPwd})
	if err != nil {
		return "", fmt.Errorf("build login info: %w", err)
	}

	// Step 6: POST /username-login → jwtToken.
	jwtToken, err := p.usernameLogin([]string{loginBaseURL, encLoginInfo, encUsername, saltData.SecretKey})
	if err != nil {
		return "", fmt.Errorf("username login: %w", err)
	}

	return jwtToken, nil
}

// --- Operation: encrypt_username ---
//
// AES-GCM encrypt plaintext with given key and IV(nonce).
// Output: base64(ciphertext + tag), tag(16字节) 拼在密文末尾.
//
// Args: [key_raw, iv_base64, plaintext] → base64 encrypted string
func (p *loginPlugin) encryptUsername(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("encrypt_username requires 3 args: key (raw), iv (base64), plaintext")
	}
	key := []byte(args[0])
	iv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}
	plaintext := []byte(args[2])

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	// GCM: output = ciphertext || tag (tag appended by default).
	ciphertext := gcm.Seal(nil, iv, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// --- Operation: get_salt ---
//
// POST {base_url}/get-app-salt with encrypted username.
// Returns JSON data portion as string.
//
// Args: [base_url, encrypted_username] → {"secretKey":"...","iv":"...","saltStr":"..."}
func (p *loginPlugin) getSalt(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New("get_salt requires 2 args: base_url, encrypted_username")
	}
	baseURL := strings.TrimRight(args[0], "/")
	encUsername := args[1]

	payload := map[string]string{"username": encUsername}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	url := baseURL + "/get-app-salt"
	log.Printf("[login-plugin] getSalt: calling GET-APP-SALT url=%s", url)
	respBody, err := p.httpPost(url, "application/json", body)
	if err != nil {
		return "", fmt.Errorf("post get-app-salt: %w", err)
	}

	log.Printf("[login-plugin] getSalt: raw response body=%s", string(respBody))

	// login.md 响应格式: {"errorCode": 0, "data": {...}}
	var resp struct {
		ErrorCode int             `json:"errorCode"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if resp.ErrorCode != 0 {
		return "", fmt.Errorf("get-app-salt failed: errorCode=%d body=%s", resp.ErrorCode, string(respBody))
	}
	if len(resp.Data) == 0 {
		return "", errors.New("get-app-salt: empty data in response")
	}

	return string(resp.Data), nil
}

// --- Operation: decrypt_salt ---
//
// AES-GCM decrypt saltStr using server-returned secretKey and iv.
//
// Args: [secret_key_base64, iv_base64, salt_str_base64] → raw decrypted string ("secretKey:bCryptSalt")
func (p *loginPlugin) decryptSalt(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("decrypt_salt requires 3 args: secret_key (base64), iv (base64), salt_str (base64)")
	}
	keyB64 := args[0]
	ivB64 := args[1]
	saltB64 := args[2]

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", fmt.Errorf("decode secret_key: %w", err)
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return "", fmt.Errorf("decode salt_str: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm decrypt: %w", err)
	}

	return string(plaintext), nil
}

// --- Operation: bcrypt_hash ---
//
// 使用完整 bcrypt salt (含 $2a$10$ 前缀) 对密码进行 hashpw.
// 与 Python bcrypt.hashpw(password.encode(), b_salt.encode()) 行为一致.
//
// Args: [password, salt_str] → hashed password string
func (p *loginPlugin) bcryptHash(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New("bcrypt_hash requires 2 args: password, salt_str")
	}
	password := []byte(args[0])
	saltStr := args[1]

	// 从完整 salt 字符串中提取 rounds/cost.
	cost := bcryptsalt.DefaultCost
	parts := strings.Split(saltStr, "$")
	if len(parts) >= 3 {
		if n, err := fmt.Sscanf(parts[2], "%d", &cost); err != nil || n != 1 {
			cost = bcryptsalt.DefaultCost
		}
	}

	hashed, err := bcryptsalt.GenerateFromPassword(password, saltStr, cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt generate: %w", err)
	}

	return string(hashed), nil
}

// --- Operation: build_login_info ---
//
// 拼接 "secretKey:hashedPassword"，然后用 secretKey+iv 做 AES-GCM 加密.
// 输出: base64(ciphertext + tag).
//
// Args: [secret_key_base64, iv_base64, hashed_password] → base64 encrypted login info
func (p *loginPlugin) buildLoginInfo(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("build_login_info requires 3 args: secret_key (base64), iv (base64), hashed_password")
	}
	keyB64 := args[0]
	ivB64 := args[1]
	hashedPwd := args[2]

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", fmt.Errorf("decode secret_key: %w", err)
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}

	loginInfo := keyB64 + ":" + hashedPwd

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	ciphertext := gcm.Seal(nil, iv, []byte(loginInfo), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// --- Operation: username_login ---
//
// POST {base_url}/username-login.
//
// Args: [base_url, login_info_encrypted, username_encrypted, secret_key_base64] → JWT token
func (p *loginPlugin) usernameLogin(args []string) (string, error) {
	if len(args) < 4 {
		return "", errors.New("username_login requires 4 args: base_url, login_info (encrypted), username (encrypted), secret_key (base64)")
	}
	baseURL := strings.TrimRight(args[0], "/")
	loginInfo := args[1]
	username := args[2]
	secretKey := args[3]

	payload := map[string]string{
		"loginInfo": loginInfo,
		"username":  username,
		"secretKey": secretKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	url := baseURL + "/username-login"
	log.Printf("[login-plugin] usernameLogin: calling USERNAME-LOGIN url=%s", url)
	respBody, err := p.httpPost(url, "application/json", body)
	if err != nil {
		return "", fmt.Errorf("post username-login: %w", err)
	}

	log.Printf("[login-plugin] usernameLogin: raw response body=%s", string(respBody))

	// login.md 响应格式: {"errorCode": 0, "data": {"jwtToken": "..."}}
	var resp struct {
		ErrorCode int             `json:"errorCode"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if resp.ErrorCode != 0 {
		return "", fmt.Errorf("username-login failed: errorCode=%d body=%s", resp.ErrorCode, string(respBody))
	}

	var data struct {
		JWTToken string `json:"jwtToken"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return string(resp.Data), nil
	}
	if data.JWTToken == "" {
		return "", errors.New("username-login: empty jwtToken in response")
	}

	return data.JWTToken, nil
}

// --- HTTP helper ---

func (p *loginPlugin) httpPost(url, contentType string, body []byte) ([]byte, error) {
	log.Printf("[login-plugin] HTTP POST %s: request body=%s", url, string(body))
	if p.client == nil {
		p.client = &http.Client{Timeout: httpTimeout}
	}
	resp, err := p.client.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		log.Printf("[login-plugin] HTTP POST %s: error=%v", url, err)
		return nil, fmt.Errorf("http post %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("[login-plugin] HTTP POST %s: response status=%d, body=%s", url, resp.StatusCode, string(respBody))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// --- AES-GCM helper (for testing random nonce mode) ---

// gcmEncryptWithRandomNonce encrypts with a random 12-byte nonce, prepending it to output.
// Used when IV is not provided externally.
func gcmEncryptWithRandomNonce(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// New is the factory function exported for the SO plugin loader.
func New() (contract.Plugin, error) {
	return &loginPlugin{
		client: &http.Client{Timeout: httpTimeout},
	}, nil
}
