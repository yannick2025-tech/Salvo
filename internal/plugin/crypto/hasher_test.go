package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

func TestHasherPluginName(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h)
	assert.Equal(t, "hmac-sha256-signer", hp.Name())
}

func TestHasherPluginCustomName(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h, WithHasherPluginName("custom-signer"))
	assert.Equal(t, "custom-signer", hp.Name())
}

func TestHasherPluginPriority(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h)
	assert.Equal(t, 2, hp.Priority())
}

func TestHasherPluginCustomPriority(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h, WithHasherPriority(15))
	assert.Equal(t, 15, hp.Priority())
}

func TestHasherPluginBefore(t *testing.T) {
	key := testKey()
	h := NewHMACSHA256(key)
	hp := NewHasherPlugin(h)

	req := &http.HTTPRequest{Body: []byte(`{"data":"value"}`)}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, hp.Before(ctx))

	sig, exists := req.Headers["X-Signature"]
	assert.True(t, exists)
	assert.NotEmpty(t, sig)

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(`{"data":"value"}`))
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestHasherPluginBeforeCustomHeader(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h, WithHasherHeaderName("X-My-Sig"))

	req := &http.HTTPRequest{Body: []byte("test")}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, hp.Before(ctx))

	_, exists := req.Headers["X-My-Sig"]
	assert.True(t, exists)
}

func TestHasherPluginBeforeNilHeaders(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h)

	req := &http.HTTPRequest{Body: []byte("test")}
	assert.Nil(t, req.Headers)

	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, hp.Before(ctx))
	assert.NotNil(t, req.Headers)
}

func TestHasherPluginAfterNoop(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, hp.After(ctx))
}

func TestHasherPluginAlgorithmName(t *testing.T) {
	h := NewHMACSHA256(testKey())
	hp := NewHasherPlugin(h)
	assert.Equal(t, "hmac-sha256", hp.AlgorithmName())
}

func TestVerifierPluginName(t *testing.T) {
	v := NewHMACSHA256(testKey())
	vp := NewVerifierPlugin(v)
	assert.Equal(t, "hmac-sha256-verifier", vp.Name())
}

func TestVerifierPluginPriority(t *testing.T) {
	v := NewHMACSHA256(testKey())
	vp := NewVerifierPlugin(v)
	assert.Equal(t, 98, vp.Priority())
}

func TestVerifierPluginBeforeNoop(t *testing.T) {
	v := NewHMACSHA256(testKey())
	vp := NewVerifierPlugin(v)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, vp.Before(ctx))
}

func TestVerifierPluginAfterValidSignature(t *testing.T) {
	key := testKey()
	v := NewHMACSHA256(key)
	vp := NewVerifierPlugin(v)

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
	require.NoError(t, vp.After(ctx))
}

func TestVerifierPluginAfterMissingHeader(t *testing.T) {
	v := NewHMACSHA256(testKey())
	vp := NewVerifierPlugin(v)

	resp := &http.HTTPResponse{
		Body:    []byte("data"),
		Headers: map[string][]string{},
	}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	err := vp.After(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing signature header")
}

func TestVerifierPluginAfterSignatureMismatch(t *testing.T) {
	v := NewHMACSHA256(testKey())
	vp := NewVerifierPlugin(v)

	resp := &http.HTTPResponse{
		Body: []byte("data"),
		Headers: map[string][]string{
			"X-Signature": {"badsignature"},
		},
	}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	err := vp.After(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature mismatch")
}

func TestHashVerifyRoundTrip(t *testing.T) {
	key := testKey()
	h := NewHMACSHA256(key)
	hp := NewHasherPlugin(h)
	vp := NewVerifierPlugin(h)

	req := &http.HTTPRequest{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Body:   []byte(`{"message":"hello"}`),
	}

	signCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, hp.Before(signCtx))

	sig := req.Headers["X-Signature"]
	resp := &http.HTTPResponse{
		Body: []byte(`{"message":"hello"}`),
		Headers: map[string][]string{
			"X-Signature": {sig},
		},
	}
	verifyCtx := plugin.NewContext(context.Background(), req)
	verifyCtx.SetResponse(resp)
	require.NoError(t, vp.After(verifyCtx))
}

func TestHasherVerifierInterfaces(t *testing.T) {
	h := NewHMACSHA256(testKey())
	var _ Hasher = h
	var _ Verifier = h
}

func TestHasherVerifierPluginInterfaces(t *testing.T) {
	h := NewHMACSHA256(testKey())
	var _ plugin.Plugin = NewHasherPlugin(h)
	var _ plugin.Plugin = NewVerifierPlugin(h)
}

func TestVerifyHex(t *testing.T) {
	h := NewHMACSHA256(testKey())
	data := []byte("test data")
	digest := h.Hash(data)
	assert.True(t, VerifyHex(h, data, digest))
	assert.False(t, VerifyHex(h, data, "invalid"))
}

func TestVerifyHexDifferentLength(t *testing.T) {
	h := NewHMACSHA256(testKey())
	assert.False(t, VerifyHex(h, []byte("data"), "abc"))
}
