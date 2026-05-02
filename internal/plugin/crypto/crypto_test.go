package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestAESGCMEncryptorInvalidKey(t *testing.T) {
	_, err := NewAESGCMEncryptor([]byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestAESGCMDecryptorInvalidKey(t *testing.T) {
	_, err := NewAESGCMDecryptor([]byte("short"))
	assert.Error(t, err)
}

func TestAESGCMEncryptorName(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey())
	require.NoError(t, err)
	assert.Equal(t, "aes-gcm-encryptor", e.Name())
}

func TestAESGCMEncryptorCustomName(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey(), WithAESGCMName("custom"))
	require.NoError(t, err)
	assert.Equal(t, "custom", e.Name())
}

func TestAESGCMEncryptorPriority(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey())
	require.NoError(t, err)
	assert.Equal(t, 5, e.Priority())
}

func TestAESGCMEncryptorCustomPriority(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey(), WithAESGCMPriority(10))
	require.NoError(t, err)
	assert.Equal(t, 10, e.Priority())
}

func TestAESGCMEncryptBefore(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey())
	require.NoError(t, err)

	req := &http.HTTPRequest{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Body:   []byte(`{"secret":"data"}`),
	}
	ctx := plugin.NewContext(context.Background(), req)

	require.NoError(t, e.Before(ctx))
	assert.NotEqual(t, `{"secret":"data"}`, string(req.Body))
	assert.Greater(t, len(req.Body), len(`{"secret":"data"`))
}

func TestAESGCMEncryptBeforeEmptyBody(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey())
	require.NoError(t, err)

	req := &http.HTTPRequest{Method: http.MethodGet, URL: "http://example.com"}
	ctx := plugin.NewContext(context.Background(), req)

	require.NoError(t, e.Before(ctx))
	assert.Nil(t, req.Body)
}

func TestAESGCMEncryptAfterNoop(t *testing.T) {
	e, err := NewAESGCMEncryptor(testKey())
	require.NoError(t, err)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, e.After(ctx))
}

func TestAESGCMDecryptorName(t *testing.T) {
	d, err := NewAESGCMDecryptor(testKey())
	require.NoError(t, err)
	assert.Equal(t, "aes-gcm-decryptor", d.Name())
}

func TestAESGCMDecryptorPriority(t *testing.T) {
	d, err := NewAESGCMDecryptor(testKey())
	require.NoError(t, err)
	assert.Equal(t, 95, d.Priority())
}

func TestAESGCMDecryptAfter(t *testing.T) {
	key := testKey()
	enc, err := NewAESGCMEncryptor(key)
	require.NoError(t, err)
	dec, err := NewAESGCMDecryptor(key)
	require.NoError(t, err)

	original := []byte(`{"hello":"world"}`)
	req := &http.HTTPRequest{Body: original}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, enc.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	require.NoError(t, dec.After(decCtx))
	assert.Equal(t, original, resp.Body)
}

func TestAESGCMDecryptAfterEmptyBody(t *testing.T) {
	d, err := NewAESGCMDecryptor(testKey())
	require.NoError(t, err)

	resp := &http.HTTPResponse{}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	require.NoError(t, d.After(ctx))
}

func TestAESGCMDecryptBeforeNoop(t *testing.T) {
	d, err := NewAESGCMDecryptor(testKey())
	require.NoError(t, err)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, d.Before(ctx))
}

func TestAESGCMDecryptAfterWrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	rand.Read(key2)

	enc, err := NewAESGCMEncryptor(key1)
	require.NoError(t, err)
	dec, err := NewAESGCMDecryptor(key2)
	require.NoError(t, err)

	req := &http.HTTPRequest{Body: []byte("test data")}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, enc.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	err = dec.After(decCtx)
	assert.Error(t, err)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey()
	plaintext := []byte("the quick brown fox jumps over the lazy dog")

	ciphertext, err := encryptAESGCM(key, plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := decryptAESGCM(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptTooShort(t *testing.T) {
	key := testKey()
	_, err := decryptAESGCM(key, []byte("short"))
	assert.Error(t, err)
}

func TestHMACSignerName(t *testing.T) {
	s := NewHMACSigner(testKey())
	assert.Equal(t, "hmac-signer", s.Name())
}

func TestHMACSignerCustomName(t *testing.T) {
	s := NewHMACSigner(testKey(), WithHMACSignerName("custom-signer"))
	assert.Equal(t, "custom-signer", s.Name())
}

func TestHMACSignerPriority(t *testing.T) {
	s := NewHMACSigner(testKey())
	assert.Equal(t, 2, s.Priority())
}

func TestHMACSignerCustomPriority(t *testing.T) {
	s := NewHMACSigner(testKey(), WithHMACSignerPriority(15))
	assert.Equal(t, 15, s.Priority())
}

func TestHMACSignerBefore(t *testing.T) {
	key := testKey()
	s := NewHMACSigner(key)

	req := &http.HTTPRequest{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Body:   []byte(`{"data":"value"}`),
	}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, s.Before(ctx))

	sig, exists := req.Headers["X-Signature"]
	assert.True(t, exists)
	assert.NotEmpty(t, sig)

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(`{"data":"value"}`))
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestHMACSignerBeforeCustomHeader(t *testing.T) {
	s := NewHMACSigner(testKey(), WithHMACHeaderName("X-My-Sig"))

	req := &http.HTTPRequest{Body: []byte("test")}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, s.Before(ctx))

	_, exists := req.Headers["X-My-Sig"]
	assert.True(t, exists)
}

func TestHMACSignerBeforeNilHeaders(t *testing.T) {
	s := NewHMACSigner(testKey())

	req := &http.HTTPRequest{Body: []byte("test")}
	assert.Nil(t, req.Headers)

	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, s.Before(ctx))
	assert.NotNil(t, req.Headers)
}

func TestHMACSignerAfterNoop(t *testing.T) {
	s := NewHMACSigner(testKey())
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, s.After(ctx))
}

func TestHMACVerifierName(t *testing.T) {
	v := NewHMACVerifier(testKey())
	assert.Equal(t, "hmac-verifier", v.Name())
}

func TestHMACVerifierPriority(t *testing.T) {
	v := NewHMACVerifier(testKey())
	assert.Equal(t, 98, v.Priority())
}

func TestHMACVerifierBeforeNoop(t *testing.T) {
	v := NewHMACVerifier(testKey())
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, v.Before(ctx))
}

func TestHMACVerifierAfterValidSignature(t *testing.T) {
	key := testKey()
	v := NewHMACVerifier(key)

	body := []byte(`{"response":"ok"}`)
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	resp := &http.HTTPResponse{
		Body: body,
		Headers: map[string][]string{
			"X-Signature": {sig},
		},
	}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	require.NoError(t, v.After(ctx))
}

func TestHMACVerifierAfterMissingHeader(t *testing.T) {
	v := NewHMACVerifier(testKey())

	resp := &http.HTTPResponse{
		Body:    []byte("data"),
		Headers: map[string][]string{},
	}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	err := v.After(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing signature header")
}

func TestHMACVerifierAfterSignatureMismatch(t *testing.T) {
	v := NewHMACVerifier(testKey())

	resp := &http.HTTPResponse{
		Body: []byte("data"),
		Headers: map[string][]string{
			"X-Signature": {"badsignature"},
		},
	}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	err := v.After(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature mismatch")
}

func TestHMACSignVerifyRoundTrip(t *testing.T) {
	key := testKey()
	signer := NewHMACSigner(key)
	verifier := NewHMACVerifier(key)

	req := &http.HTTPRequest{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Body:   []byte(`{"message":"hello"}`),
	}

	signCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, signer.Before(signCtx))

	sig := req.Headers["X-Signature"]
	resp := &http.HTTPResponse{
		Body: []byte(`{"message":"hello"}`),
		Headers: map[string][]string{
			"X-Signature": {sig},
		},
	}
	verifyCtx := plugin.NewContext(context.Background(), req)
	verifyCtx.SetResponse(resp)
	require.NoError(t, verifier.After(verifyCtx))
}

func TestPluginInterfaceCompliance(t *testing.T) {
	var _ plugin.Plugin = &AESGCMEncryptor{}
	var _ plugin.Plugin = &AESGCMDecryptor{}
	var _ plugin.Plugin = &HMACSigner{}
	var _ plugin.Plugin = &HMACVerifier{}
}
