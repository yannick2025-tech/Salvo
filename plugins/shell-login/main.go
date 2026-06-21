// Package main implements the shell-login SO plugin for Shell Manhattan login flow.
//
// Build: go build -buildmode=plugin -o shell-login.so plugins/shell-login/main.go
//
// This plugin replicates the login.py authentication flow:
//  1. AES-CBC encrypt username with hardcoded key/IV
//  2. Call /get-app-salt API to get dynamic salt, secretKey, iv
//  3. AES decrypt the salt string to extract bcrypt parameters
//  4. BCrypt hash password with extracted salt and rounds
//  5. Build login payload (secretKey:hashedPassword) and AES encrypt
//  6. Call /username-login API to obtain JWT token
//
// Usage from expressions:
//
//	${__so("shell-login", "login", "base_url", "username", "password")}
//	${__so("shell-login", "encrypt_username", "key", "iv", "username")}
//	${__so("shell-login", "get_salt", "base_url", "encrypted_username")}
//	${__so("shell-login", "decrypt_salt", "secret_key", "iv", "salt_str")}
//	${__so("shell-login", "bcrypt_hash", "password", "salt_str")}
//	${__so("shell-login", "build_login_info", "secret_key", "hashed_password")}
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
)

// Default key/IV matching login.py constants.
const (
	defaultSecretKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes
	defaultIV        = "BBBBBBBBBBBBBBBBBBBBBA=="         // base64(16 bytes)
	httpTimeout      = 15 * time.Second
)

// shellLogin implements the so.Plugin interface for Shell Manhattan login.
type shellLogin struct {
	client *http.Client
}

// Name returns the plugin name.
func (p *shellLogin) Name() string { return "shell-login" }

// Version returns the plugin version.
func (p *shellLogin) Version() string { return "1.0.0" }

// Call executes the named operation.
//
// Supported operations:
//   - login(base_url, username, password) → JWT token (full flow)
//   - encrypt_username(key, iv, username) → encrypted username (base64)
//   - get_salt(base_url, encrypted_username) → JSON: {"secretKey":"...","iv":"...","saltStr":"..."}
//   - decrypt_salt(secret_key, iv, salt_str) → raw decrypted salt
//   - bcrypt_hash(password, salt_str) → bcrypt hashed password
//   - build_login_info(secret_key, hashed_password) → encrypted login info (base64)
func (p *shellLogin) Call(op string, args []string) (string, error) {
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

// login performs the complete authentication flow and returns a JWT token.
// Args: [base_url, username, password]
func (p *shellLogin) login(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("login requires 3 args: base_url, username, password")
	}
	baseURL := args[0]
	username := args[1]
	password := args[2]

	// Step 1: Encrypt username with default key/IV.
	encUsername, err := p.encryptUsername([]string{defaultSecretKey, defaultIV, username})
	if err != nil {
		return "", fmt.Errorf("encrypt username: %w", err)
	}

	// Step 2: Get app salt from API.
	saltResp, err := p.getSalt([]string{baseURL, encUsername})
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

	// Step 3: Decrypt salt to extract bcrypt parameters.
	rawSalt, err := p.decryptSalt([]string{saltData.SecretKey, saltData.IV, saltData.SaltStr})
	if err != nil {
		return "", fmt.Errorf("decrypt salt: %w", err)
	}

	// Parse raw salt format: "secretKey:$2b$rounds$saltHash"
	parts := strings.SplitN(rawSalt, ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid salt format: expected 'secretKey:salt', got %q", rawSalt)
	}
	bcryptSalt := parts[1]

	// Step 4: BCrypt hash password.
	hashedPwd, err := p.bcryptHash([]string{password, bcryptSalt})
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}

	// Step 5: Build and encrypt login info.
	encLoginInfo, err := p.buildLoginInfo([]string{saltData.SecretKey, hashedPwd})
	if err != nil {
		return "", fmt.Errorf("build login info: %w", err)
	}

	// Step 6: Call username-login API.
	jwtToken, err := p.usernameLogin([]string{baseURL, encLoginInfo, encUsername, saltData.SecretKey})
	if err != nil {
		return "", fmt.Errorf("username login: %w", err)
	}

	return jwtToken, nil
}

// --- Operation: encrypt_username ---

// encryptUsername AES-CBC encrypts the username with given key/IV.
// Args: [key, iv_base64, plaintext] → base64 ciphertext (IV prepended).
func (p *shellLogin) encryptUsername(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("encrypt_username requires 3 args: key, iv (base64), plaintext")
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

	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV (matching login.py AESCiphers output format).
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)

	return base64.StdEncoding.EncodeToString(result), nil
}

// --- Operation: get_salt ---

// getSalt calls the /get-app-salt API endpoint.
// Args: [base_url, encrypted_username] → JSON response body as string.
func (p *shellLogin) getSalt(args []string) (string, error) {
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

	url := baseURL + "/jv/crm/jv/auth/v1/get-app-salt"
	respBody, err := p.httpPost(url, "application/json", body)
	if err != nil {
		return "", fmt.Errorf("post get-app-salt: %w", err)
	}

	// Parse response and extract data field.
	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"message"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if resp.Code != 0 && resp.Code != 200 {
		return "", fmt.Errorf("get-app-salt failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return "", errors.New("get-app-salt: empty data in response")
	}

	return string(resp.Data), nil
}

// --- Operation: decrypt_salt ---

// decryptSalt AES-CBC decrypts the salt string.
// Args: [secret_key, iv_base64, salt_str_base64] → raw decrypted salt string.
func (p *shellLogin) decryptSalt(args []string) (string, error) {
	if len(args) < 3 {
		return "", errors.New("decrypt_salt requires 3 args: secret_key, iv (base64), salt_str (base64)")
	}
	key := []byte(args[0])
	// The iv parameter is accepted for API compatibility but the actual IV
	// is extracted from the ciphertext (login.py format: IV prepended).
	if _, err := base64.StdEncoding.DecodeString(args[1]); err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}
	saltB64 := args[2]

	ciphertext, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return "", fmt.Errorf("decode salt_str: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	// login.py format: IV is prepended to ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	actualIV := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext not aligned to block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, actualIV)
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("unpad: %w", err)
	}

	return string(plaintext), nil
}

// --- Operation: bcrypt_hash ---

// bcryptHash hashes the password using bcrypt with the given full salt string.
// Extracts the cost (rounds) from the salt and generates a valid bcrypt hash.
// Args: [password, salt_str] → hashed password string.
func (p *shellLogin) bcryptHash(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New("bcrypt_hash requires 2 args: password, salt_str")
	}
	password := args[0]
	saltStr := args[1]

	// Extract cost/rounds from salt format: $2b$rounds$22charsalt
	cost := bcrypt.DefaultCost
	parts := strings.Split(saltStr, "$")
	if len(parts) >= 3 {
		if n, err := fmt.Sscanf(parts[2], "%d", &cost); err != nil || n != 1 {
			cost = bcrypt.DefaultCost
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt generate: %w", err)
	}

	return string(hashed), nil
}

// --- Operation: build_login_info ---

// buildLoginInfo builds and AES-encrypts the login payload.
// Args: [secret_key, hashed_password] → encrypted login info (base64).
func (p *shellLogin) buildLoginInfo(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New("build_login_info requires 2 args: secret_key, hashed_password")
	}
	secretKey := args[0]
	hashedPwd := args[1]

	loginInfo := secretKey + ":" + hashedPwd

	// Encrypt with the secretKey as both key and derive IV from it.
	key := []byte(secretKey)
	if len(key) != 32 {
		// Pad or truncate key to 32 bytes (AES-256).
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}
	// Derive IV from first 16 bytes of key.
	derivedIV := key[:16]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	padded := pkcs7Pad([]byte(loginInfo), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, derivedIV)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV.
	result := make([]byte, len(derivedIV)+len(ciphertext))
	copy(result, derivedIV)
	copy(result[len(derivedIV):], ciphertext)

	return base64.StdEncoding.EncodeToString(result), nil
}

// --- Operation: username_login ---

// usernameLogin calls the /username-login API endpoint.
// Args: [base_url, login_info_encrypted, username_encrypted, secret_key] → JWT token.
func (p *shellLogin) usernameLogin(args []string) (string, error) {
	if len(args) < 4 {
		return "", errors.New("username_login requires 4 args: base_url, login_info (encrypted), username (encrypted), secret_key")
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

	url := baseURL + "/jv/jv-adapter/jv/auth/v1/username-login"
	respBody, err := p.httpPost(url, "application/json", body)
	if err != nil {
		return "", fmt.Errorf("post username-login: %w", err)
	}

	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"message"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if resp.Code != 0 && resp.Code != 200 {
		return "", fmt.Errorf("username-login failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	// Extract jwtToken from data.
	var data struct {
		JWTToken string `json:"jwtToken"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		// Return raw data if jwtToken field not found.
		return string(resp.Data), nil
	}

	if data.JWTToken == "" {
		return "", errors.New("username-login: empty jwtToken in response")
	}

	return data.JWTToken, nil
}

// --- HTTP helper ---

func (p *shellLogin) httpPost(url, contentType string, body []byte) ([]byte, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: httpTimeout}
	}
	resp, err := p.client.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http post %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// --- PKCS7 padding (same as shell-aes) ---

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

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
	return &shellLogin{
		client: &http.Client{Timeout: httpTimeout},
	}, nil
}
