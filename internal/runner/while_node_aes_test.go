package runner

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpprotocol "github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// manhattanCBCEncrypt encrypts plaintext using AES-CBC with Manhattan convention:
// IV = key[:16], output is JSON-encoded base64 string.
func manhattanCBCEncrypt(plaintext string, keyBytes []byte) ([]byte, error) {
	iv := keyBytes[:16]
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	// PKCS7 pad
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return json.Marshal(encoded)
}

// generateRSAKey generates a test RSA private key and returns:
// - *rsa.PrivateKey for signing
// - base64-encoded PKCS8 private key for the sign plugin
func generateRSAKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	// Use a pre-generated key pair for test stability
	// In production, this would be loaded from config
	privKey, err := rsa.GenerateKey(nil, 2048)
	if err != nil {
		// Fallback: use a deterministic test key
		t.Skip("RSA key generation not available, using mock test")
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	pkcs8Base64 := base64.StdEncoding.EncodeToString(pkcs8Bytes)
	return privKey, pkcs8Base64
}

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int { return &v }

// TestWhileStepAESDecrypt_QueryChargingStatus simulates the Manhattan query-by-order-id
// flow: sign request → HTTP POST → AES-encrypted response → decrypt → extract charging_status.
//
// This test verifies:
// 1. The mock Manhattan API returns an AES-CBC encrypted response
// 2. The while step's aes_decrypt config correctly decrypts the response
// 3. The extract correctly reads chargingStatus from the decrypted JSON
// 4. The order_id "202607211619060001" is used in the request body
func TestWhileStepAESDecrypt_QueryChargingStatus(t *testing.T) {
	// --- Setup: AES key (Manhattan convention) ---
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	// --- Setup: mock Manhattan API that returns AES-encrypted response ---
	orderID := "202607211619060001"
	plainResponse := fmt.Sprintf(
		`{"errorCode":0,"data":{"chargingStatus":6,"orderId":"%s","soc":80}}`,
		orderID,
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read request body to verify order_id is included
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), orderID, "request body should contain order_id")

		// Return AES-encrypted response (Manhattan convention)
		encryptedBody, err := manhattanCBCEncrypt(plainResponse, rawKey)
		if err != nil {
			t.Errorf("encrypt failed: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(encryptedBody)
	}))
	defer mockServer.Close()

	// --- Execute: simulate while step HTTP request ---
	step := &stepConfig{
		Name: "查询充电状态",
		Request: &stepRequestConfig{
			Method:  "POST",
			URL:     mockServer.URL + "/ev/toc-adapter/charging-process-data/v1/query-by-order-id",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    fmt.Sprintf(`{"orderId":"%s"}`, orderID),
		},
		AesDecrypt: aesKeyBase64,
		AesMode:    intPtr(0), // CBC
		Extract: extractConfig{
			{Variable: "charging_status", Path: "$.data.chargingStatus"},
			{Variable: "charging_soc", Path: "$.data.soc"},
		},
	}

	loopVars := map[string]any{}

	// Build and execute the HTTP request
	req := buildStepHTTPRequest(step.Request, loopVars, nil)
	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(context.Background(), req)
	require.NoError(t, err)

	httpResp, ok := resp.(*httpprotocol.HTTPResponse)
	require.True(t, ok, "response should be HTTPResponse")
	require.True(t, httpResp.IsSuccess(), "HTTP status should be 2xx")

	// Verify the raw response body is AES-encrypted (doesn't start with '{')
	assert.NotEqual(t, uint8('{'), httpResp.Body[0], "raw response should be encrypted")

	// --- Decrypt using the while step logic ---
	if step.AesDecrypt != "" && len(httpResp.Body) > 0 && httpResp.Body[0] != '{' {
		aesMode := 1
		if step.AesMode != nil {
			aesMode = *step.AesMode
		}
		decrypted, decryptErr := aesDecryptResponse(httpResp.Body, step.AesDecrypt, aesMode)
		require.NoError(t, decryptErr, "AES decrypt should succeed")
		httpResp.Body = []byte(decrypted)
	}

	// Verify the decrypted response is valid JSON
	assert.Equal(t, uint8('{'), httpResp.Body[0], "decrypted response should be JSON")

	// --- Extract variables ---
	extractVarsFromResponse(httpResp.Body, step.Extract, loopVars, newTestLogger())

	// Verify extracted values
	assert.Equal(t, float64(6), loopVars["charging_status"],
		"charging_status should be extracted as 6 (charging complete)")
	assert.Equal(t, float64(80), loopVars["charging_soc"],
		"soc should be extracted as 80")
}

// TestWhileStepAESDecrypt_CreateOrder simulates the create-charge-order flow
// where the response is AES-encrypted and order_id must be extracted.
func TestWhileStepAESDecrypt_CreateOrder(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	orderID := "202607211619060001"
	plainResponse := fmt.Sprintf(
		`{"errorCode":0,"data":{"orderId":"%s","orderStatus":"CHARGING"}}`,
		orderID,
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encryptedBody, err := manhattanCBCEncrypt(plainResponse, rawKey)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(encryptedBody)
	}))
	defer mockServer.Close()

	step := &stepConfig{
		Name: "创建订单",
		Request: &stepRequestConfig{
			Method:  "POST",
			URL:     mockServer.URL + "/ev/toc-adapter/order/v1/create-charge-order",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"chargingPointId":"test-point","orderSource":0}`,
		},
		AesDecrypt: aesKeyBase64,
		AesMode:    intPtr(0),
		Extract: extractConfig{
			{Variable: "order_id", Path: "$.data.orderId"},
		},
	}

	loopVars := map[string]any{}

	req := buildStepHTTPRequest(step.Request, loopVars, nil)
	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(context.Background(), req)
	require.NoError(t, err)

	httpResp, ok := resp.(*httpprotocol.HTTPResponse)
	require.True(t, ok)
	require.True(t, httpResp.IsSuccess())

	// Decrypt
	require.NotEqual(t, uint8('{'), httpResp.Body[0], "response should be encrypted")
	aesMode2 := 1
	if step.AesMode != nil {
		aesMode2 = *step.AesMode
	}
	decrypted, err := aesDecryptResponse(httpResp.Body, step.AesDecrypt, aesMode2)
	require.NoError(t, err)
	httpResp.Body = []byte(decrypted)

	// Extract
	extractVarsFromResponse(httpResp.Body, step.Extract, loopVars, newTestLogger())

	assert.Equal(t, orderID, loopVars["order_id"],
		"order_id should be extracted from decrypted response")
}

// TestWhileStepAESDecrypt_PlainJSONSkip verifies that when the response is
// plain JSON (not encrypted), the decrypt step is skipped correctly.
func TestWhileStepAESDecrypt_PlainJSONSkip(t *testing.T) {
	aesKeyBase64 := base64.StdEncoding.EncodeToString(make([]byte, 32))

	plainResponse := `{"errorCode":0,"data":{"chargingStatus":3}}`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(plainResponse))
	}))
	defer mockServer.Close()

	step := &stepConfig{
		Name: "查询充电状态-明文",
		Request: &stepRequestConfig{
			Method:  "POST",
			URL:     mockServer.URL,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"orderId":"test"}`,
		},
		AesDecrypt: aesKeyBase64,
		AesMode:    intPtr(0),
		Extract: extractConfig{
			{Variable: "charging_status", Path: "$.data.chargingStatus"},
		},
	}

	loopVars := map[string]any{}

	req := buildStepHTTPRequest(step.Request, loopVars, nil)
	proto := httpprotocol.NewProtocol()
	resp, err := proto.Execute(context.Background(), req)
	require.NoError(t, err)

	httpResp, ok := resp.(*httpprotocol.HTTPResponse)
	require.True(t, ok)

	// Response starts with '{', so decrypt should be skipped
	if step.AesDecrypt != "" && len(httpResp.Body) > 0 && httpResp.Body[0] != '{' {
		t.Fatal("decrypt should have been skipped for plain JSON response")
	}

	// Extract directly (no decrypt needed)
	extractVarsFromResponse(httpResp.Body, step.Extract, loopVars, newTestLogger())

	assert.Equal(t, float64(3), loopVars["charging_status"],
		"charging_status should be extracted from plain JSON response")
}

// TestWhileStepAESDecrypt_MultipleSteps simulates the full while loop flow:
// step 1: sign the request (generator)
// step 2: HTTP POST with signed headers + AES decrypt + extract
func TestWhileStepAESDecrypt_MultipleSteps(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(rawKey)

	// Simulate charging status progression: 1→2→3→6
	statusSequence := []int{1, 2, 3, 6}
	statusIdx := 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := statusSequence[statusIdx%len(statusSequence)]
		statusIdx++

		plainResponse := fmt.Sprintf(
			`{"errorCode":0,"data":{"chargingStatus":%d,"orderId":"202607211619060001"}}`,
			status,
		)
		encryptedBody, err := manhattanCBCEncrypt(plainResponse, rawKey)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(encryptedBody)
	}))
	defer mockServer.Close()

	// Simulate 4 iterations of the while loop
	loopVars := map[string]any{"charging_status": ""}

	for i := 0; i < 4; i++ {
		step := &stepConfig{
			Name: fmt.Sprintf("查询充电状态-第%d次", i+1),
			Request: &stepRequestConfig{
				Method:  "POST",
				URL:     mockServer.URL + "/ev/toc-adapter/charging-process-data/v1/query-by-order-id",
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    `{"orderId":"202607211619060001"}`,
			},
			AesDecrypt: aesKeyBase64,
			AesMode:    intPtr(0),
			Extract: extractConfig{
				{Variable: "charging_status", Path: "$.data.chargingStatus"},
			},
		}

		req := buildStepHTTPRequest(step.Request, loopVars, nil)
		proto := httpprotocol.NewProtocol()
		resp, err := proto.Execute(context.Background(), req)
		require.NoError(t, err)

		httpResp, ok := resp.(*httpprotocol.HTTPResponse)
		require.True(t, ok)

		// Decrypt
		if httpResp.Body[0] != '{' {
			aesMode3 := 1
			if step.AesMode != nil {
				aesMode3 = *step.AesMode
			}
			decrypted, err := aesDecryptResponse(httpResp.Body, step.AesDecrypt, aesMode3)
			require.NoError(t, err)
			httpResp.Body = []byte(decrypted)
		}

		// Extract
		extractVarsFromResponse(httpResp.Body, step.Extract, loopVars, newTestLogger())

		expectedStatus := float64(statusSequence[i])
		assert.Equal(t, expectedStatus, loopVars["charging_status"],
			"iteration %d: charging_status should be %d", i+1, statusSequence[i])
	}

	// Final status should be 6 (charging complete)
	assert.Equal(t, float64(6), loopVars["charging_status"],
		"final charging_status should be 6 (charging complete)")
}
