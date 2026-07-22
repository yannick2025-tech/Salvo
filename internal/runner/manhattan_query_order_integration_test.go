package runner

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sharedcrypto "github.com/yannick2025-tech/Salvo/plugins/shared/crypto"
)

// TestManhattan_QueryChargingOrderByID 集成测试：查询充电订单状态
//
// 完整流程：
//  1. 登录获取 JWT (login 插件流程)
//  2. 获取 C端 Token (含 token, rsa_pri_key, aes_key)
//  3. 签名请求体 {"orderId":202607211619060001} (RSA 签名)
//  4. HTTP POST 到 /ev/toc-adapter/charging-process-data/v1/query-by-order-id
//  5. AES-CBC 解密响应
//  6. 打印加密响应 + 解密后的 JSON
//
// 运行方式：
//
//	go test ./internal/runner/ -run TestManhattan_QueryChargingOrderByID -v -count=1
//
// 注意：此测试调用真实 UAT 环境，需要网络连通
func TestManhattan_QueryChargingOrderByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var (
		baseurl         = "https://uat-manhattan.shell.com.cn"
		gatewayurl      = "http://gateway.ev-shell.com.cn" // 与 card.yaml 一致
		envType         = "uat1"
		phoneNo         = "18936879143"
		password        = "Abcd1234!@#$"
		orderID         = "202607211619060001"
		saltBaseURL     = baseurl + "/jv/tob-adapter/tob-adapter-application/crm/jv/auth/v1"
		loginBaseURL    = baseurl + "/jv/tob-adapter/jv/auth/v1"
		apiGetToken     = "/ev-tools/session/toc-session/get"
		apiChargingData = "/ev/toc-adapter/charging-process-data/v1/query-by-order-id"
	)

	// 如果环境变量 MANHATTAN_GATEWAY_URL 设置了，使用它覆盖默认值
	if gwURL := os.Getenv("MANHATTAN_GATEWAY_URL"); gwURL != "" {
		gatewayurl = gwURL
		t.Logf("使用自定义 gateway URL: %s", gatewayurl)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// === Step 1: 登录获取 JWT ===
	t.Log("=== Step 1: 登录获取 JWT ===")
	jwtToken, err := loginGetJWT(t, client, saltBaseURL, loginBaseURL, phoneNo, password)
	require.NoError(t, err, "登录失败")
	require.NotEmpty(t, jwtToken, "JWT token 不应为空")
	t.Logf("JWT Token: %s...%s", jwtToken[:20], jwtToken[len(jwtToken)-10:])

	// === Step 2: 获取 C端 Token (token, rsa_pri_key, aes_key) ===
	t.Log("=== Step 2: 获取 C端 Token ===")
	token, rsaPriKey, aesKey, err := getTOCSession(client, gatewayurl+apiGetToken, envType, phoneNo)
	require.NoError(t, err, "获取 C端 Token 失败")
	require.NotEmpty(t, token, "token 不应为空")
	require.NotEmpty(t, rsaPriKey, "rsa_pri_key 不应为空")
	require.NotEmpty(t, aesKey, "aes_key 不应为空")
	t.Logf("Token: %s...%s", token[:20], token[len(token)-10:])
	t.Logf("AES Key: %s", aesKey)

	// === Step 3: 签名请求体 ===
	t.Log("=== Step 3: 签名请求体 ===")
	requestBody := fmt.Sprintf(`{"orderId":%s}`, orderID)
	sign, ts, err := signSHA1withRSAInline(rsaPriKey, requestBody)
	require.NoError(t, err, "签名失败")
	t.Logf("请求体: %s", requestBody)
	t.Logf("签名结果: sign=%s..., ts=%s", sign[:20], ts)

	// === Step 4: HTTP POST 查询充电状态 ===
	t.Log("=== Step 4: HTTP POST 查询充电状态 ===")
	respBody, statusCode, err := doPostSigned(client, gatewayurl+apiChargingData, token, sign, ts, requestBody)
	require.NoError(t, err, "HTTP 请求失败")
	t.Logf("HTTP Status: %d", statusCode)

	// === Step 5: 打印原始响应 + AES 解密 ===
	t.Log("=== Step 5: 分析响应 ===")

	// 打印原始响应体
	t.Logf("原始响应体 (前 500 字符): %s", truncateString(string(respBody), 500))

	// 判断是否加密
	if len(respBody) > 0 && respBody[0] != '{' {
		t.Log("响应体是 AES 加密的 (首字符不是 '{')")

		// 解密
		decrypted, decryptErr := aesDecryptResponse(respBody, aesKey, 0)
		if decryptErr != nil {
			t.Errorf("AES 解密失败: %v", decryptErr)
		} else {
			// 美化打印解密后的 JSON
			var prettyJSON map[string]any
			if jsonErr := json.Unmarshal([]byte(decrypted), &prettyJSON); jsonErr == nil {
				prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
				t.Logf("解密后响应 (格式化 JSON):\n%s", string(prettyBytes))
			} else {
				t.Logf("解密后响应 (原始):\n%s", decrypted)
			}

			// 提取关键字段
			var data struct {
				ErrorCode int `json:"errorCode"`
				Data      struct {
					ChargingStatus int    `json:"chargingStatus"`
					OrderID        string `json:"orderId"`
				} `json:"data"`
			}
			if parseErr := json.Unmarshal([]byte(decrypted), &data); parseErr == nil {
				t.Logf("提取结果: errorCode=%d, chargingStatus=%d, orderId=%s",
					data.ErrorCode, data.Data.ChargingStatus, data.Data.OrderID)
				assert.Equal(t, 0, data.ErrorCode, "errorCode 应为 0")
			}
		}
	} else {
		t.Log("响应体是明文 JSON (首字符是 '{')")
		var prettyJSON map[string]any
		if jsonErr := json.Unmarshal(respBody, &prettyJSON); jsonErr == nil {
			prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
			t.Logf("明文响应 (格式化 JSON):\n%s", string(prettyBytes))
		}
	}
}

// --- Login flow helpers ---

// loginGetJWT 执行完整的登录流程获取 JWT token
func loginGetJWT(t *testing.T, client *http.Client, saltBaseURL, loginBaseURL, username, password string) (string, error) {
	t.Helper()

	// Step 1: AES-GCM 加密用户名
	secretKeyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ivB64 := "BBBBBBBBBBBBBBBBBBBBBA=="

	keyRaw, err := base64.StdEncoding.DecodeString(secretKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode secret key: %w", err)
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", fmt.Errorf("decode iv: %w", err)
	}

	encUsername, err := gcmEncryptInline(username, keyRaw, iv)
	if err != nil {
		return "", fmt.Errorf("encrypt username: %w", err)
	}
	t.Logf("加密用户名: %s", encUsername)

	// Step 2: POST /get-app-salt
	saltPayload, _ := json.Marshal(map[string]string{"username": encUsername})
	saltRespBody, err := httpPostHelper(client, saltBaseURL+"/get-app-salt", "application/json", saltPayload)
	if err != nil {
		return "", fmt.Errorf("get-app-salt: %w", err)
	}
	t.Logf("get-app-salt 响应: %s", truncateString(string(saltRespBody), 300))

	var saltResp struct {
		ErrorCode int             `json:"errorCode"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(saltRespBody, &saltResp); err != nil {
		return "", fmt.Errorf("parse salt response: %w", err)
	}
	if saltResp.ErrorCode != 0 {
		return "", fmt.Errorf("get-app-salt errorCode=%d", saltResp.ErrorCode)
	}

	var saltData struct {
		SecretKey string `json:"secretKey"`
		IV        string `json:"iv"`
		SaltStr   string `json:"saltStr"`
	}
	if err := json.Unmarshal(saltResp.Data, &saltData); err != nil {
		return "", fmt.Errorf("parse salt data: %w", err)
	}

	// Step 3: AES-GCM 解密 saltStr
	saltKey, _ := base64.StdEncoding.DecodeString(saltData.SecretKey)
	saltIV, _ := base64.StdEncoding.DecodeString(saltData.IV)
	rawSalt, err := gcmDecryptInline(saltData.SaltStr, saltKey, saltIV)
	if err != nil {
		return "", fmt.Errorf("decrypt salt: %w", err)
	}
	parts := strings.SplitN(rawSalt, ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid salt format: %q", rawSalt)
	}
	bcryptSalt := parts[1]

	// Step 4: bcrypt hash
	hashedPwd, err := bcryptHashPassword(password, bcryptSalt)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}

	// Step 5: AES-GCM 加密 loginInfo
	loginInfo := saltData.SecretKey + ":" + hashedPwd
	encLoginInfo, err := gcmEncryptInline(loginInfo, saltKey, saltIV)
	if err != nil {
		return "", fmt.Errorf("encrypt login info: %w", err)
	}

	// Step 6: POST /username-login
	loginPayload, _ := json.Marshal(map[string]string{
		"loginInfo": encLoginInfo,
		"username":  encUsername,
		"secretKey": saltData.SecretKey,
	})
	loginRespBody, err := httpPostHelper(client, loginBaseURL+"/username-login", "application/json", loginPayload)
	if err != nil {
		return "", fmt.Errorf("username-login: %w", err)
	}
	t.Logf("username-login 响应: %s", truncateString(string(loginRespBody), 300))

	var loginResp struct {
		ErrorCode int `json:"errorCode"`
		Data      struct {
			JWTToken string `json:"jwtToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRespBody, &loginResp); err != nil {
		return "", fmt.Errorf("parse login response: %w", err)
	}
	if loginResp.ErrorCode != 0 {
		return "", fmt.Errorf("username-login errorCode=%d", loginResp.ErrorCode)
	}

	return loginResp.Data.JWTToken, nil
}

// --- Crypto helpers ---

// gcmEncryptInline AES-GCM 加密（与 login 插件逻辑一致，使用 16 字节 nonce）
func gcmEncryptInline(plaintext string, key, nonce []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCMWithNonceSize(block, len(nonce))
	if err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// gcmDecryptInline AES-GCM 解密（使用 16 字节 nonce）
func gcmDecryptInline(ciphertextB64 string, key, nonce []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCMWithNonceSize(block, len(nonce))
	if err != nil {
		return "", err
	}
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// signSHA1withRSAInline RSA-SHA1 签名（与 sign 插件逻辑一致）
func signSHA1withRSAInline(prvKeyBase64, body string) (signature, ts string, err error) {
	pk, err := base64.StdEncoding.DecodeString(prvKeyBase64)
	if err != nil {
		return "", "", fmt.Errorf("decode private key: %w", err)
	}
	private, err := x509.ParsePKCS8PrivateKey(pk)
	if err != nil {
		return "", "", fmt.Errorf("parse PKCS8 key: %w", err)
	}
	privateKey, ok := private.(*rsa.PrivateKey)
	if !ok {
		return "", "", fmt.Errorf("key is not RSA private key")
	}

	hash := sha256.Sum256([]byte(body))
	hexHash := hex.EncodeToString(hash[:])
	nowMs := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	shaBody := hexHash + nowMs

	h := crypto.SHA1.New()
	h.Write([]byte(shaBody))
	hashed := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA1, hashed)
	if err != nil {
		return "", "", fmt.Errorf("RSA sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nowMs, nil
}

// --- HTTP helpers ---

// getTOCSession 获取 C端 Token
func getTOCSession(client *http.Client, url, envType, phoneNo string) (token, rsaPriKey, aesKey string, err error) {
	payload, _ := json.Marshal(map[string]any{
		"envType":       envType,
		"phoneNo":       phoneNo,
		"expireMinutes": 1440,
	})
	respBody, err := httpPostHelper(client, url, "application/json", payload)
	if err != nil {
		return "", "", "", fmt.Errorf("http post: %w", err)
	}

	var resp struct {
		ErrorCode int `json:"errorCode"`
		Data      struct {
			Token  string `json:"token"`
			PriKey string `json:"priKey"`
			AesKey string `json:"aesKey"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", "", "", fmt.Errorf("parse response: %w", err)
	}
	if resp.ErrorCode != 0 {
		return "", "", "", fmt.Errorf("errorCode=%d", resp.ErrorCode)
	}

	return resp.Data.Token, resp.Data.PriKey, resp.Data.AesKey, nil
}

// doPostSigned 发送带签名头的 HTTP POST 请求
func doPostSigned(client *http.Client, url, token, sign, ts, body string) ([]byte, int, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("x-ev-api-sign", sign)
	req.Header.Set("x-ev-api-timestamp", ts)

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}

// httpPostHelper 通用 HTTP POST
func httpPostHelper(client *http.Client, url, contentType string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// bcryptHashPassword 对密码进行 bcrypt hash
func bcryptHashPassword(password, salt string) (string, error) {
	cost := 10
	parts := strings.Split(salt, "$")
	if len(parts) >= 3 {
		if n, err := fmt.Sscanf(parts[2], "%d", &cost); err != nil || n != 1 {
			cost = 10
		}
	}
	result, err := sharedcrypto.GenerateFromPassword([]byte(password), salt, cost)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
