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

func TestShellAESName(t *testing.T) {
	p := &shellAES{}
	assert.Equal(t, "shell-aes", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
}

func TestShellAESEncryptDecryptRoundTrip(t *testing.T) {
	p := &shellAES{}
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

func TestShellAESEncryptDeterministic(t *testing.T) {
	p := &shellAES{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	// Same IV + same data = same result (CBC with fixed IV is deterministic).
	result1, err := p.Call("encrypt", []string{key, iv, "test data"})
	require.NoError(t, err)

	result2, err := p.Call("encrypt", []string{key, iv, "test data"})
	require.NoError(t, err)
	assert.Equal(t, result1, result2, "CBC with fixed IV should produce the same ciphertext")
}

func TestShellAESEncryptDiffIV(t *testing.T) {
	p := &shellAES{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	iv1 := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))
	iv2 := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnop"))

	result1, err := p.Call("encrypt", []string{key, iv1, "test data"})
	require.NoError(t, err)

	result2, err := p.Call("encrypt", []string{key, iv2, "test data"})
	require.NoError(t, err)
	assert.NotEqual(t, result1, result2, "Different IVs should produce different ciphertexts")
}

func TestShellAESDecryptInvalidBase64(t *testing.T) {
	p := &shellAES{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	_, err := p.Call("decrypt", []string{key, iv, "not-valid-base64!!"})
	assert.Error(t, err)
}

func TestShellAESUnknownOp(t *testing.T) {
	p := &shellAES{}
	_, err := p.Call("unknown", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestShellAESEncryptWrongArgCount(t *testing.T) {
	p := &shellAES{}
	_, err := p.Call("encrypt", []string{"key", "iv"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestShellAESDecryptWrongArgCount(t *testing.T) {
	p := &shellAES{}
	_, err := p.Call("decrypt", []string{"key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 3 args")
}

func TestShellAESInvalidIV(t *testing.T) {
	p := &shellAES{}
	_, err := p.Call("encrypt", []string{"key", "invalid-iv!!!", "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding iv")
}

func TestShellAESShortKey(t *testing.T) {
	p := &shellAES{}
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))
	_, err := p.Call("encrypt", []string{"short", iv, "data"})
	assert.Error(t, err)
}

// TestShellAESBuildAndLoad builds the .so and loads it via the SO loader.
func TestShellAESBuildAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	soPath := "/tmp/shell-aes-test.so"

	// Build the plugin.
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soPath, ".")
	cmd.Dir = "/Users/xiongyang/Desktop/home/code/snailx/plugins/shell-aes"
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
	assert.Equal(t, "shell-aes", inst.Name())
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

// TestShellAESWithLoginPyKey verifies compatibility with login.py's hardcoded key/IV.
func TestShellAESWithLoginPyKey(t *testing.T) {
	p := &shellAES{}
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

// TestShellAESConcurrent verifies concurrent access is safe.
func TestShellAESConcurrent(t *testing.T) {
	p := &shellAES{}
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

// TestShellAESLargeData tests encryption/decryption of larger payloads.
func TestShellAESLargeData(t *testing.T) {
	p := &shellAES{}
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	iv := base64.StdEncoding.EncodeToString([]byte("1234567890123456"))

	largeData := strings.Repeat("A", 4096)
	encResult, err := p.Call("encrypt", []string{key, iv, largeData})
	require.NoError(t, err)

	decResult, err := p.Call("decrypt", []string{key, iv, encResult})
	require.NoError(t, err)
	assert.Equal(t, largeData, decResult)
}