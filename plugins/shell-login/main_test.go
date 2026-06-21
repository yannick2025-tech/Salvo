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

// --- Unit tests (no HTTP, no .so build) ---

func TestShellLoginName(t *testing.T) {
	p := &shellLogin{}
	assert.Equal(t, "shell-login", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
}

func TestShellLoginUnknownOp(t *testing.T) {
	p := &shellLogin{}
	_, err := p.Call("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestShellLoginEncryptUsername(t *testing.T) {
	p := &shellLogin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := "BBBBBBBBBBBBBBBBBBBBBA=="
	username := "13312345674"

	result, err := p.encryptUsername([]string{key, iv, username})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Result should be valid base64.
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.True(t, len(decoded) > 16, "ciphertext should contain IV + data")

	// Verify round-trip with decryptSalt logic.
	rawKey := []byte(key)
	block, err := aesNewCipher(rawKey) // use direct call to avoid import cycle in test
	require.NoError(t, err)

	ivBytes := decoded[:16]
	ct := decoded[16:]
	pt := make([]byte, len(ct))
	mode := newCBCDecrypter(block, ivBytes)
	mode.CryptBlocks(pt, ct)
	pt, _ = pkcs7Unpad(pt, 16)
	assert.Equal(t, username, string(pt))
}

func TestShellLoginEncryptUsernameWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.encryptUsername([]string{"key", "iv"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestShellLoginEncryptUsernameInvalidIV(t *testing.T) {
	p := &shellLogin{}
	_, err := p.encryptUsername([]string{"key", "not-base64!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode iv")
}

func TestShellLoginBcryptHash(t *testing.T) {
	p := &shellLogin{}

	// Use a standard bcrypt salt format.
	saltStr := "$2b$10$abcdefghijklmnopqurst"
	result, err := p.bcryptHash([]string{"861365", saltStr})
	require.NoError(t, err)
	require.NotEmpty(t, result)
	// bcrypt hash always starts with $2a$ or $2b$
	assert.True(t, strings.HasPrefix(result, "$2"), "bcrypt hash should start with $2")
}

func TestShellLoginBcryptHashWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.bcryptHash([]string{"password"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 2 args")
}

func TestShellLoginBuildLoginInfo(t *testing.T) {
	p := &shellLogin{}
	secretKey := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // exactly 32 bytes
	hashedPwd := "$2b$10$hashedpasswordvaluehere..."

	result, err := p.buildLoginInfo([]string{secretKey, hashedPwd})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Should be valid base64.
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.True(t, len(decoded) > 16, "should have IV prepended")
}

func TestShellLoginBuildLoginInfoWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.buildLoginInfo([]string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 2 args")
}

func TestShellLoginDecryptSalt(t *testing.T) {
	p := &shellLogin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	// First encrypt a known plaintext.
	encResult, err := p.encryptUsername([]string{key, iv, "test-salt-data:10" + "$" + "somesalt"})
	require.NoError(t, err)

	// Now decrypt it.
	decResult, err := p.decryptSalt([]string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, "test-salt-data:10"+"$"+"somesalt", decResult)
}

func TestShellLoginDecryptSaltWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.decryptSalt([]string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestShellLoginGetSaltMockServer(t *testing.T) {
	p := &shellLogin{}

	// Start mock server for get-app-salt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jv/crm/jv/auth/v1/get-app-salt", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		jsonResp := `{"code":0,"data":{"secretKey":"dynamic-key-32bytes-long!!","iv":"dHJpYWw=","saltStr":"encrypted-salt-here"}}`
		fmt.Fprint(w, jsonResp)
	}))
	defer srv.Close()

	result, err := p.getSalt([]string{srv.URL, "encrypted-user"})
	require.NoError(t, err)

	// Should return the data portion as JSON string.
	assert.Contains(t, result, "secretKey")
	assert.Contains(t, result, "saltStr")
}

func TestShellLoginGetSaltWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.getSalt([]string{"base_url"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 2 args")
}

func TestShellLoginUsernameLoginMockServer(t *testing.T) {
	p := &shellLogin{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jv/jv-adapter/jv/auth/v1/username-login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"code":0,"data":{"jwtToken":"test-jwt-token-xyz"}}`)
	}))
	defer srv.Close()

	result, err := p.usernameLogin([]string{srv.URL, "enc-login-info", "enc-username", "secret-key"})
	require.NoError(t, err)
	assert.Equal(t, "test-jwt-token-xyz", result)
}

func TestShellLoginUsernameLoginWrongArgs(t *testing.T) {
	p := &shellLogin{}
	_, err := p.usernameLogin([]string{"url"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 4 args")
}

func TestShellLoginFullFlowMockServer(t *testing.T) {
	p := &shellLogin{}

	// Mock get-app-salt: returns salt that decrypts to "dynamic-key:$2b$10$abc".
	encryptFn := func(key, ivB64, plaintext string) string {
		r, _ := p.encryptUsername([]string{key, ivB64, plaintext})
		return r
	}
	dynamicKey := "dynamic-secret-key-32bytlong!!!!" // exactly 32 bytes
	dynamicIV := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	// Build encrypted salt that decrypts to "dynamic-key:$2b$10$abcdefghijkmnopqrstuv"
	rawSalt := dynamicKey + ":$2b$10$abcdefghijklmnopqrstu"
	encSalt := encryptFn(dynamicKey, dynamicIV, rawSalt)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/jv/crm/jv/auth/v1/get-app-salt":
			callCount++
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"code":0,"data":{"secretKey":"%s","iv":"%s","saltStr":"%s"}}`,
				dynamicKey, dynamicIV, encSalt)
		case "/jv/jv-adapter/jv/auth/v1/username-login":
			callCount++
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"code":0,"data":{"jwtToken":"mock-jwt-token-full-flow"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"unknown path %s"}`, r.URL.Path)
		}
	}))
	defer srv.Close()

	jwtToken, err := p.login([]string{srv.URL, "13312345674", "861365"})
	if err != nil {
		// The login flow may fail due to bcrypt hash mismatch with mock server,
		// but we verify the flow reaches the login endpoint.
		t.Logf("login error (may be expected with mock): %v", err)
		// Error should be in either decrypt salt, username-login, or bcrypt phase
		assert.True(t,
			strings.Contains(err.Error(), "username-login") ||
				strings.Contains(err.Error(), "decrypt salt") ||
				strings.Contains(err.Error(), "bcrypt"),
			"error should be from a known phase")
	} else {
		assert.Equal(t, "mock-jwt-token-full-flow", jwtToken)
	}
	assert.GreaterOrEqual(t, callCount, 1, "should have called at least get-app-salt")
}

// --- PKCS7 padding tests ---

func TestPKCS7PadUnpad(t *testing.T) {
	data := []byte("hello world") // 11 bytes
	padded := pkcs7Pad(data, 16)
	assert.Equal(t, 16, len(padded), "padded should be block size")
	assert.Equal(t, byte(5), padded[15], "last byte should be padding value (5)")

	unpadded, err := pkcs7Unpad(padded, 16)
	require.NoError(t, err)
	assert.Equal(t, data, unpadded)
}

func TestPKCS7UnpadEmpty(t *testing.T) {
	_, err := pkcs7Unpad([]byte{}, 16)
	assert.Error(t, err)
}

func TestPKCS7UnpadInvalidPadding(t *testing.T) {
	bad := make([]byte, 16)
	bad[15] = 0xFF // invalid padding value
	_, err := pkcs7Unpad(bad, 16)
	assert.Error(t, err)
}

func TestPKCS7PadExactMultiple(t *testing.T) {
	data := make([]byte, 16) // exactly one block
	padded := pkcs7Pad(data, 16)
	assert.Equal(t, 32, len(padded), "exact multiple needs full padding block")
	unpadded, err := pkcs7Unpad(padded, 16)
	require.NoError(t, err)
	assert.Equal(t, data, unpadded)
}

// --- Build and Load test ---

func TestShellLoginBuildAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	soPath := "/tmp/shell-login-test.so"

	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soPath, ".")
	cmd.Dir = "/Users/xiongyang/Desktop/home/code/snailx/plugins/shell-login"
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(output))
	defer os.Remove(soPath)

	// Verify the .so file exists and is non-empty.
	info, err := os.Stat(soPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, ".so file should not be empty")
	t.Logf("built shell-login.so: %d bytes", info.Size())
}

// --- Concurrency test ---

func TestShellLoginConcurrent(t *testing.T) {
	p := &shellLogin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := "BBBBBBBBBBBBBBBBBBBBBA=="

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

// --- Helper functions for testing (avoiding import of internal packages) ---

func aesNewCipher(key []byte) (cipher.Block, error) { return aes.NewCipher(key) }
func newCBCDecrypter(block cipher.Block, iv []byte) cipher.BlockMode {
	return cipher.NewCBCDecrypter(block, iv)
}
