package runner

import (
	"crypto"
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
)

// TestManhattan_QueryChargingOrderByID 集成测试：查询充电订单状态
//
// 简化流程（C端用户无需登录管理平台，仅需手机号获取 TOC Session）：
//  1. 获取 C端 Token (含 token, rsa_pri_key, aes_key)
//  2. 签名请求体 {"orderId":...} (RSA-SHA1 签名)
//  3. HTTP POST 到 /ev/toc-adapter/charging-process-data/v1/query-by-order-id
//  4. AES-GCM 解密响应 (mode=1，UAT 环境当前使用 GCM)
//  5. 打印解密后的 JSON 并校验关键字段
//
// 运行方式：
//
//	go test ./internal/runner/ -run TestManhattan_QueryChargingOrderByID -v -count=1
//
// 环境变量：
//
//	MANHATTAN_GATEWAY_URL  覆盖 gateway URL (默认 http://gateway.ev-shell.com.cn)
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
		phoneNo         = "15550000000" // C端小程序用户，与 card.yaml 的 user_charger.phone 一致
		orderID         = "202607211619060001"
		apiGetToken     = "/ev-tools/session/toc-session/get"
		apiChargingData = "/ev/toc-adapter/charging-process-data/v1/query-by-order-id"
	)

	if gwURL := os.Getenv("MANHATTAN_GATEWAY_URL"); gwURL != "" {
		gatewayurl = gwURL
		t.Logf("使用自定义 gateway URL: %s", gatewayurl)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// === Step 1: 获取 C端 Token (token, rsa_pri_key, aes_key) ===
	t.Log("=== Step 1: 获取 C端 Token ===")
	token, rsaPriKey, aesKey, err := getTOCSession(client, gatewayurl+apiGetToken, envType, phoneNo)
	require.NoError(t, err, "获取 C端 Token 失败")
	require.NotEmpty(t, token, "token 不应为空")
	require.NotEmpty(t, rsaPriKey, "rsa_pri_key 不应为空")
	require.NotEmpty(t, aesKey, "aes_key 不应为空")
	t.Logf("Token: %s...%s", token[:20], token[len(token)-10:])
	t.Logf("AES Key: %s", aesKey)

	// === Step 2: 签名请求体 ===
	t.Log("=== Step 2: 签名请求体 ===")
	requestBody := fmt.Sprintf(`{"orderId":%s}`, orderID)
	sign, ts, err := signSHA1withRSAInline(rsaPriKey, requestBody)
	require.NoError(t, err, "签名失败")
	t.Logf("请求体: %s", requestBody)
	t.Logf("签名结果: sign=%s..., ts=%s", sign[:20], ts)

	// === Step 3: HTTP POST 查询充电状态 ===
	t.Log("=== Step 3: HTTP POST 查询充电状态 ===")
	respBody, statusCode, err := doPostSigned(client, baseurl+apiChargingData, token, sign, ts, requestBody)
	require.NoError(t, err, "HTTP 请求失败")
	t.Logf("HTTP Status: %d", statusCode)

	// === Step 4: 解密 + 校验响应 ===
	t.Log("=== Step 4: 分析响应 ===")
	t.Logf("原始响应体 (前 500 字符): %s", truncateStr(string(respBody), 500))

	if len(respBody) > 0 && respBody[0] != '{' {
		t.Log("响应体是 AES 加密的 (首字符不是 '{')")

		// mode=1: AES-GCM（UAT 环境当前使用 GCM）
		decrypted, decryptErr := aesDecryptResponse(respBody, aesKey, 1)
		if decryptErr != nil {
			t.Errorf("AES-GCM 解密失败: %v", decryptErr)
		} else {
			var prettyJSON map[string]any
			if jsonErr := json.Unmarshal([]byte(decrypted), &prettyJSON); jsonErr == nil {
				prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
				t.Logf("解密后响应 (格式化 JSON):\n%s", string(prettyBytes))
			} else {
				t.Logf("解密后响应 (原始):\n%s", decrypted)
			}

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

// --- Crypto helpers (integration test only) ---

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

// --- HTTP helpers (integration test only) ---

// getTOCSession 获取 C端 Token（无需 JWT 认证，仅需手机号）
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

// truncateStr 截断字符串用于日志输出
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
