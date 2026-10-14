package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	defaultSecretKey = func() string {
		raw, _ := base64.StdEncoding.DecodeString(defaultSecretKeyB64)
		return string(raw)
	}()
	defaultIV = defaultIVB64
)

// --- Unit tests ---

func TestLoginName(t *testing.T) {
	p := &loginPlugin{}
	assert.Equal(t, "login", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
}

func TestLoginUnknownOp(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.Call("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

// --- encrypt_username (AES-GCM) ---

func TestLoginEncryptUsername(t *testing.T) {
	p := &loginPlugin{}
	key := defaultSecretKey // 32 bytes raw
	iv := defaultIV         // base64(12 bytes nonce for GCM)
	username := "18936870000"

	result, err := p.encryptUsername([]string{key, iv, username})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Result should be valid base64.
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	// GCM output = ciphertext(11) + tag(16) = 27+ bytes for 11-byte input
	assert.True(t, len(decoded) > 16, "ciphertext should contain data + tag")

	// Verify round-trip decrypt.
	rawKey := []byte(key)
	ivBytes, _ := base64.StdEncoding.DecodeString(iv)

	block, err := aes.NewCipher(rawKey)
	require.NoError(t, err)
	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	require.NoError(t, err)

	pt, err := gcm.Open(nil, ivBytes, decoded, nil)
	require.NoError(t, err)
	assert.Equal(t, username, string(pt))
}

func TestLoginEncryptUsernameWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.encryptUsername([]string{"key", "iv"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestLoginEncryptUsernameInvalidIV(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.encryptUsername([]string{"key", "not-base64!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode iv")
}

// --- get_salt ---

func TestLoginGetSaltMockServer(t *testing.T) {
	p := &loginPlugin{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/get-app-salt", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		jsonResp := `{"errorCode":0,"data":{"secretKey":"dGVzdC1rZXk=","iv":"dGVzdC12","saltStr":"encrypted-salt-here"}}`
		fmt.Fprint(w, jsonResp)
	}))
	defer srv.Close()

	result, err := p.getSalt([]string{srv.URL, "encrypted-user"})
	require.NoError(t, err)
	assert.Contains(t, result, "secretKey")
	assert.Contains(t, result, "saltStr")
}

func TestLoginGetSaltWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.getSalt([]string{"base_url"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 2 args")
}

// --- decrypt_salt (AES-GCM) ---

func TestLoginDecryptSalt(t *testing.T) {
	p := &loginPlugin{}
	keyRaw := defaultSecretKey // 32 bytes raw
	ivB64 := defaultIV

	// First encrypt with raw key (encrypt_username takes raw key).
	encResult, err := p.encryptUsername([]string{keyRaw, ivB64, "test-salt-data" + ":" + "$2a$10$abcdefghijklmnopqurstuv"})
	require.NoError(t, err)

	// Decrypt: decrypt_salt takes base64-encoded key.
	keyB64 := base64.StdEncoding.EncodeToString([]byte(keyRaw))
	decResult, err := p.decryptSalt([]string{keyB64, ivB64, encResult})
	require.NoError(t, err)
	assert.Equal(t, "test-salt-data:$2a$10$abcdefghijklmnopqurstuv", decResult)
}

func TestLoginDecryptSaltWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.decryptSalt([]string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

// --- bcrypt_hash ---

func TestLoginBcryptHash(t *testing.T) {
	p := &loginPlugin{}
	saltStr := "$2a$10$abcdefghijklmnopqurstuv" // valid bcrypt salt format (22 chars)
	result, err := p.bcryptHash([]string{"Xb09@#47124", saltStr})
	require.NoError(t, err)
	require.NotEmpty(t, result)
	assert.True(t, strings.HasPrefix(result, "$2"), "bcrypt hash should start with $2")
}

func TestLoginBcryptHashWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.bcryptHash([]string{"password"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 2 args")
}

// --- build_login_info (AES-GCM) ---

func TestLoginBuildLoginInfo(t *testing.T) {
	p := &loginPlugin{}
	// Use base64-encoded key and iv matching login.md format.
	keyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	ivB64 := base64.StdEncoding.EncodeToString(make([]byte, 16)) // 16 bytes for GCM nonce
	hashedPwd := "$2a$10$hashedpasswordvaluehere..."

	result, err := p.buildLoginInfo([]string{keyB64, ivB64, hashedPwd})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.True(t, len(decoded) > 16, "should have ciphertext + tag")
}

func TestLoginBuildLoginInfoWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.buildLoginInfo([]string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

// --- username_login ---

func TestLoginUsernameLoginMockServer(t *testing.T) {
	p := &loginPlugin{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/username-login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"errorCode":0,"data":{"jwtToken":"test-jwt-token-xyz"}}`)
	}))
	defer srv.Close()

	result, err := p.usernameLogin([]string{srv.URL, "enc-login-info", "enc-username", "secret-key"})
	require.NoError(t, err)
	assert.Equal(t, "test-jwt-token-xyz", result)
}

func TestLoginUsernameLoginWrongArgs(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.usernameLogin([]string{"url"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 4 args")
}

// --- Full flow mock server test ---

func TestLoginFullFlowMockServer(t *testing.T) {
	p := &loginPlugin{}

	dynamicKeyRaw := make([]byte, 32)
	for i := range dynamicKeyRaw {
		dynamicKeyRaw[i] = byte(i + 1)
	}
	dynamicKeyB64 := base64.StdEncoding.EncodeToString(dynamicKeyRaw)
	dynamicIVRaw := make([]byte, 16)
	for i := range dynamicIVRaw {
		dynamicIVRaw[i] = byte(i + 100)
	}
	dynamicIVB64 := base64.StdEncoding.EncodeToString(dynamicIVRaw)

	rawSalt := dynamicKeyB64 + ":$2a$10$abcdefghijklmnopqurstuv"

	block, _ := aes.NewCipher(dynamicKeyRaw)
	gcm, _ := cipher.NewGCMWithNonceSize(block, 16)
	encSaltBytes := gcm.Seal(nil, dynamicIVRaw, []byte(rawSalt), nil)
	encSaltB64 := base64.StdEncoding.EncodeToString(encSaltBytes)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/get-app-salt":
			callCount++
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"errorCode":0,"data":{"secretKey":"%s","iv":"%s","saltStr":"%s"}}`,
				dynamicKeyB64, dynamicIVB64, encSaltB64)
		case "/username-login":
			callCount++
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"errorCode":0,"data":{"jwtToken":"mock-jwt-token-full-flow"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"unknown path %s"}`, r.URL.Path)
		}
	}))
	defer srv.Close()

	jwtToken, err := p.login([]string{srv.URL, srv.URL, "18936870000", "Xb09@#47124"})
	require.NoError(t, err)
	assert.Equal(t, "mock-jwt-token-full-flow", jwtToken)
	assert.Equal(t, 2, callCount, "should have called both get-app-salt and username-login")
}

// --- Login arg count check ---

func TestLoginArgCount(t *testing.T) {
	p := &loginPlugin{}
	_, err := p.login([]string{"url", "user", "pwd"}) // only 3 args, need 4
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 4 args")
}

// --- Build and Load test ---

func TestLoginBuildAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	soPath := "/tmp/login-test.so"

	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soPath, ".")
	cmd.Dir = "/Users/xiongyang/Desktop/home/code/snailx/plugins/login"
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(output))
	defer os.Remove(soPath)

	info, err := os.Stat(soPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, ".so file should not be empty")
	t.Logf("built login.so: %d bytes", info.Size())
}

// --- Concurrency test ---

func TestLoginConcurrent(t *testing.T) {
	p := &loginPlugin{}
	key := defaultSecretKey
	iv := defaultIV

	done := make(chan struct{}, 20)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			_, err := p.encryptUsername([]string{key, iv, fmt.Sprintf("user-%d", idx)})
			assert.NoError(t, err)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
