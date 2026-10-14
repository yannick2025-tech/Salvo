// Package main - sign SO plugin for Salvo
//
// 功能：RSA 签名插件，业务使用中的HTTP请求签名逻辑
// 编译：go build -buildmode=plugin -o sign.so main.go
//
// 调用语法：${__so("sign","sign", rsaPriKey, requestBody)}
// 返回：JSON {"sign":"base64签名","ts":"毫秒时间戳"}
//
// 签名算法：
//  1. SHA256(requestBody) → hex 编码
//  2. 拼接: hexHash + timestampMs
//  3. SHA1withRSA(拼接字符串, rsaPriKey) → base64
package main

import (
	"bytes"
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
	"strconv"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/plugin/so/contract"
)

// signPlugin implements the so.Plugin interface for RSA signing.
type signPlugin struct{}

func (p *signPlugin) Name() string    { return "sign" }
func (p *signPlugin) Version() string { return "1.0.0" }

func (p *signPlugin) Call(operation string, args []string) (string, error) {
	switch operation {
	case "sign":
		return sign(args...)
	case "need_sign":
		return needSign(args...)
	case "check_charge_time":
		return checkChargeTime(args...)
	case "now_ms":
		return nowMs(args...)
	case "sign_post":
		return signPost(args...)
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

// New is the factory function exported for the SO plugin loader.
func New() (contract.Plugin, error) {
	return &signPlugin{}, nil
}

// sign 执行 RSA 签名
// args[0]: rsaPriKey - RSA 私钥（base64 编码的 PKCS8 格式）
// args[1]: body - 请求体 JSON 字符串
func sign(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("sign requires 2 args: rsaPriKey, body")
	}
	rsaPriKey := args[0]
	body := args[1]

	// Step 1: SHA256 hash of request body → hex encode
	hash := sha256.Sum256([]byte(body))
	hexHash := hex.EncodeToString(hash[:])

	// Step 2: Append timestamp (milliseconds)
	nowMs := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	shaBody := hexHash + nowMs

	// Step 3: SHA1withRSA sign
	signature, err := signSHA1withRSA(shaBody, rsaPriKey)
	if err != nil {
		return "", fmt.Errorf("sign failed: %v", err)
	}

	// Return JSON with sign and timestamp
	result := map[string]string{
		"sign": signature,
		"ts":   nowMs,
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// needSign 检查 API 路径是否在签名列表中
// args[0]: signApisJson - 签名 API 列表的 JSON 数组字符串，如 ["/ev/toc-adapter/order/v1/xxx"]
// args[1]: apiPath - 当前 API 路径，如 /ev/toc-adapter/order/v1/xxx
// 返回: "true" 或 "false"
func needSign(args ...string) (string, error) {
	if len(args) < 2 {
		return "false", nil
	}
	signApisJson := args[0]
	apiPath := args[1]

	if signApisJson == "" || signApisJson == "[]" {
		return "false", nil
	}

	var apis []string
	if err := json.Unmarshal([]byte(signApisJson), &apis); err != nil {
		return "false", nil
	}

	for _, api := range apis {
		if api == apiPath {
			return "true", nil
		}
	}
	return "false", nil
}

// checkChargeTime 检查充电时长是否已到达
// args[0]: chargeStartTime - 充电开始时间的毫秒时间戳字符串
// args[1]: chargeTime - 充电时长(秒)字符串
// 返回: "true" 表示已到达，"false" 表示未到达
func checkChargeTime(args ...string) (string, error) {
	if len(args) < 2 {
		return "false", nil
	}
	var startTimeMs int64
	var chargeTimeSec int64
	if _, err := fmt.Sscanf(args[0], "%d", &startTimeMs); err != nil {
		return "false", nil
	}
	if _, err := fmt.Sscanf(args[1], "%d", &chargeTimeSec); err != nil {
		return "false", nil
	}
	nowMs := time.Now().UnixNano() / 1000000
	targetMs := startTimeMs + chargeTimeSec*1000
	if nowMs >= targetMs {
		return "true", nil
	}
	return "false", nil
}

// nowMs 返回当前毫秒时间戳
// 调用：${__so("sign","now_ms")}
func nowMs(args ...string) (string, error) {
	return strconv.FormatInt(time.Now().UnixNano()/1000000, 10), nil
}

// signPost 签名 + HTTP POST 一体化（用于 while 循环内部无法使用 generator 的场景）
// args[0]: rsaPriKey - RSA 私钥
// args[1]: url - 完整请求 URL
// args[2]: token - Authorization token
// args[3]: body - 请求体 JSON 字符串
// 返回：HTTP 响应体原始 JSON 字符串
func signPost(args ...string) (string, error) {
	if len(args) < 4 {
		return "", fmt.Errorf("sign_post requires 4 args: rsaPriKey, url, token, body")
	}
	rsaPriKey := args[0]
	url := args[1]
	token := args[2]
	body := args[3]

	// 签名
	hash := sha256.Sum256([]byte(body))
	hexHash := hex.EncodeToString(hash[:])
	nowMs := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	shaBody := hexHash + nowMs
	signature, err := signSHA1withRSA(shaBody, rsaPriKey)
	if err != nil {
		return "", fmt.Errorf("sign failed: %v", err)
	}

	// HTTP POST
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("x-ev-api-sign", signature)
	req.Header.Set("x-ev-api-timestamp", nowMs)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %v", err)
	}
	return string(respBody), nil
}

// signSHA1withRSA 使用 RSA 私钥进行 SHA1withRSA 签名
// 与 snailx base/http.go 的 SignSHA1withRSA 函数完全一致
func signSHA1withRSA(content string, prvKey string) (string, error) {
	pk, err := base64.StdEncoding.DecodeString(prvKey)
	if err != nil {
		return "", fmt.Errorf("decode private key failed: %v", err)
	}

	private, err := x509.ParsePKCS8PrivateKey(pk)
	if err != nil {
		return "", fmt.Errorf("parse PKCS8 key failed: %v", err)
	}

	privateKey, ok := private.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("key is not RSA private key")
	}

	h := crypto.Hash.New(crypto.SHA1)
	h.Write([]byte(content))
	hashed := h.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA1, hashed)
	if err != nil {
		return "", fmt.Errorf("RSA sign failed: %v", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}
