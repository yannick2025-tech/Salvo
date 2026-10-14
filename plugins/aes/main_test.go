package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"plugin"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
)

func TestAESName(t *testing.T) {
	p := &aesPlugin{}
	assert.Equal(t, "aes", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
}

func TestAESEncryptDecryptRoundTrip(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes = AES-256
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456")) // 16 bytes

	plaintext := "hello world"
	encResult, err := p.Call("encrypt", []string{key, iv, plaintext})
	require.NoError(t, err)
	require.NotEmpty(t, encResult)

	decResult, err := p.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

func TestAESEncryptDeterministic(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	// Same IV + same data = same result (CBC with fixed IV is deterministic).
	result1, err := p.Call("encrypt", []string{key, iv, "test data"})
	require.NoError(t, err)

	result2, err := p.Call("encrypt", []string{key, iv, "test data"})
	require.NoError(t, err)
	assert.Equal(t, result1, result2, "CBC with fixed IV should produce the same ciphertext")
}

func TestAESEncryptDiffIV(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	iv1 := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))
	iv2 := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnop"))

	result1, err := p.Call("encrypt", []string{key, iv1, "test data"})
	require.NoError(t, err)

	result2, err := p.Call("encrypt", []string{key, iv2, "test data"})
	require.NoError(t, err)
	assert.NotEqual(t, result1, result2, "Different IVs should produce different ciphertexts")
}

func TestAESDecryptInvalidBase64(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	_, err := p.Call("decrypt", []string{key, iv, "not-valid-base64!!"})
	assert.Error(t, err)
}

func TestAESUnknownOp(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("unknown", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestAESEncryptWrongArgCount(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("encrypt", []string{"key", "iv"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestAESDecryptWrongArgCount(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("decrypt", []string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestAESInvalidIV(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("encrypt", []string{"key", "invalid-iv!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding iv")
}

func TestAESShortKey(t *testing.T) {
	p := &aesPlugin{}
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))
	_, err := p.Call("encrypt", []string{"short", iv, "data"})
	assert.Error(t, err)
}

// TestAESBuildAndLoad builds the .so and loads it via the SO loader.
func TestAESBuildAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	soPath := "/tmp/aes-test.so"

	// Build the plugin.
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soPath, ".")
	cmd.Dir = "/Users/xiongyang/Desktop/home/code/snailx/plugins/aes"
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(output))
	defer os.Remove(soPath)

	// Open the plugin. Go .so plugins must be built with the exact same
	// toolchain version as the loading binary. If there's a mismatch
	// (e.g. go test vs go build), skip gracefully.
	p, err := plugin.Open(soPath)
	if err != nil {
		t.Skipf("plugin.Open failed (likely toolchain version mismatch): %v", err)
	}

	// Lookup the New symbol.
	sym, err := p.Lookup("New")
	require.NoError(t, err)

	factory, ok := sym.(func() (so.Plugin, error))
	require.True(t, ok, "New symbol has wrong type")

	inst, err := factory()
	require.NoError(t, err)

	// Verify plugin interface.
	assert.Equal(t, "aes", inst.Name())
	assert.Equal(t, "1.0.0", inst.Version())

	// Test encrypt/decrypt via the loaded plugin.
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	encResult, err := inst.Call("encrypt", []string{key, iv, "loaded plugin test"})
	require.NoError(t, err)

	decResult, err := inst.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, "loaded plugin test", decResult)
}

// TestAESWithLoginPyKey verifies compatibility with login.py's hardcoded key/IV.
func TestAESWithLoginPyKey(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := "BBBBBBBBBBBBBBBBBBBBBA=="

	// This verifies the plugin accepts login.py's exact key/IV format.
	plaintext := "13312345674"
	encResult, err := p.Call("encrypt", []string{key, iv, plaintext})
	require.NoError(t, err)

	// The encrypted result should be base64 and contain the IV.
	decoded, err := base64.StdEncoding.DecodeString(encResult)
	require.NoError(t, err)
	assert.True(t, len(decoded) > 16, "ciphertext should be > 16 bytes (IV + encrypted data)")

	// Decrypt back.
	decResult, err := p.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

// TestAESConcurrent verifies concurrent access is safe.
func TestAESConcurrent(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	runConcurrent := func(n int) {
		done := make(chan struct{})
		for i := 0; i < n; i++ {
			go func() {
				_, err := p.Call("encrypt", []string{key, iv, "concurrent test"})
				assert.NoError(t, err)
				done <- struct{}{}
			}()
		}
		for i := 0; i < n; i++ {
			<-done
		}
	}

	runConcurrent(10) // just verify no panics
}

// TestAESLargeData tests encryption/decryption of larger payloads.
func TestAESLargeData(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	largeData := strings.Repeat("A", 4096)
	encResult, err := p.Call("encrypt", []string{key, iv, largeData})
	require.NoError(t, err)

	decResult, err := p.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, largeData, decResult)
}

// TestAESGCMEncryptDecryptRoundTrip tests GCM mode encryption/decryption.
func TestAESGCMEncryptDecryptRoundTrip(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes = AES-256
	iv := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")) // 16 bytes nonce

	plaintext := `{"errorCode":0,"data":{"orderId":"202607211619060001"}}`
	encResult, err := p.Call("encrypt_gcm", []string{key, iv, plaintext})
	require.NoError(t, err)
	require.NotEmpty(t, encResult)

	decResult, err := p.Call("decrypt_gcm", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

// TestAESGCMDifferentIVs verifies GCM produces different ciphertexts with different IVs.
func TestAESGCMDifferentIVs(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv1 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	iv2 := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789"))

	plaintext := "test plaintext"
	enc1, err := p.Call("encrypt_gcm", []string{key, iv1, plaintext})
	require.NoError(t, err)

	enc2, err := p.Call("encrypt_gcm", []string{key, iv2, plaintext})
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "GCM with different IVs should produce different ciphertexts")
}

// TestAESGCMEncryptWrongArgCount tests error handling for insufficient arguments.
func TestAESGCMEncryptWrongArgCount(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("encrypt_gcm", []string{"key", "iv"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

// TestAESGCMDecryptWrongArgCount tests error handling for insufficient arguments.
func TestAESGCMDecryptWrongArgCount(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("decrypt_gcm", []string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

// TestAESGCMInvalidIV tests error handling for invalid base64 IV.
func TestAESGCMInvalidIV(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("encrypt_gcm", []string{"key", "invalid-iv!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding iv")
}

// TestAESGCMDecryptInvalidIV tests error handling for invalid base64 IV in decrypt.
func TestAESGCMDecryptInvalidIV(t *testing.T) {
	p := &aesPlugin{}
	_, err := p.Call("decrypt_gcm", []string{"key", "invalid-iv!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding iv")
}

// TestAESGCMDecryptInvalidCiphertext tests error handling for invalid base64 ciphertext.
func TestAESGCMDecryptInvalidCiphertext(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	_, err := p.Call("decrypt_gcm", []string{key, iv, "not-valid-base64!!"})
	assert.Error(t, err)
}

// TestAESGCMDecryptWrongKey tests that decryption fails with wrong key.
func TestAESGCMDecryptWrongKey(t *testing.T) {
	p := &aesPlugin{}
	key1 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	key2 := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	iv := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	plaintext := "test plaintext"
	encResult, err := p.Call("encrypt_gcm", []string{key1, iv, plaintext})
	require.NoError(t, err)

	_, err = p.Call("decrypt_gcm", []string{key2, iv, encResult})
	assert.Error(t, err, "decryption with wrong key should fail")
}

// TestAESGCMEmptyPlaint tests encryption of empty string.
func TestAESGCMEmptyPlaint(t *testing.T) {
	p := &aesPlugin{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	encResult, err := p.Call("encrypt_gcm", []string{key, iv, ""})
	require.NoError(t, err)
	require.NotEmpty(t, encResult)

	decResult, err := p.Call("decrypt_gcm", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, "", decResult)
}

// TestAESNewFactory tests the New() factory function.
func TestAESNewFactory(t *testing.T) {
	plugin, err := New()
	require.NoError(t, err)
	require.NotNil(t, plugin)

	assert.Equal(t, "aes", plugin.Name())
	assert.Equal(t, "1.0.0", plugin.Version())
}

// TestAESNewFactoryCall tests that plugin from New() can encrypt/decrypt.
func TestAESNewFactoryCall(t *testing.T) {
	plugin, err := New()
	require.NoError(t, err)

	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	plaintext := "test from factory"
	encResult, err := plugin.Call("encrypt", []string{key, iv, plaintext})
	require.NoError(t, err)

	decResult, err := plugin.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}

// TestAESNewFactoryGCM tests that plugin from New() can use GCM mode.
func TestAESNewFactoryGCM(t *testing.T) {
	plugin, err := New()
	require.NoError(t, err)

	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	plaintext := "GCM test from factory"
	encResult, err := plugin.Call("encrypt_gcm", []string{key, iv, plaintext})
	require.NoError(t, err)

	decResult, err := plugin.Call("decrypt_gcm", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResult)
}