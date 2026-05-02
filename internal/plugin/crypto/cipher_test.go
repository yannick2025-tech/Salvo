package crypto

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

func newTestAES(t *testing.T, mode Mode) *AES {
	t.Helper()
	a, err := NewAES(testKey(), WithMode(mode))
	require.NoError(t, err)
	return a
}

func TestEncryptorPluginName(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)
	assert.Equal(t, "aes-256-gcm-encryptor", ep.Name())
}

func TestEncryptorPluginCustomName(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a, WithEncryptorPluginName("custom"))
	assert.Equal(t, "custom", ep.Name())
}

func TestEncryptorPluginPriority(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)
	assert.Equal(t, 5, ep.Priority())
}

func TestEncryptorPluginCustomPriority(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a, WithEncryptorPriority(10))
	assert.Equal(t, 10, ep.Priority())
}

func TestEncryptorPluginBefore(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)

	req := &http.HTTPRequest{Body: []byte(`{"secret":"data"}`)}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(ctx))
	assert.NotEqual(t, `{"secret":"data"}`, string(req.Body))
}

func TestEncryptorPluginBeforeEmptyBody(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)

	req := &http.HTTPRequest{}
	ctx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(ctx))
	assert.Nil(t, req.Body)
}

func TestEncryptorPluginAfterNoop(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, ep.After(ctx))
}

func TestDecryptorPluginName(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	dp := NewDecryptorPlugin(a)
	assert.Equal(t, "aes-256-gcm-decryptor", dp.Name())
}

func TestDecryptorPluginPriority(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	dp := NewDecryptorPlugin(a)
	assert.Equal(t, 95, dp.Priority())
}

func TestDecryptorPluginAfterGCM(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)
	dp := NewDecryptorPlugin(a)

	original := []byte(`{"hello":"world"}`)
	req := &http.HTTPRequest{Body: original}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	require.NoError(t, dp.After(decCtx))
	assert.Equal(t, original, resp.Body)
}

func TestDecryptorPluginAfterCBC(t *testing.T) {
	a := newTestAES(t, ModeCBC)
	ep := NewEncryptorPlugin(a)
	dp := NewDecryptorPlugin(a)

	original := []byte(`{"hello":"cbc"}`)
	req := &http.HTTPRequest{Body: original}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	require.NoError(t, dp.After(decCtx))
	assert.Equal(t, original, resp.Body)
}

func TestDecryptorPluginAfterCTR(t *testing.T) {
	a := newTestAES(t, ModeCTR)
	ep := NewEncryptorPlugin(a)
	dp := NewDecryptorPlugin(a)

	original := []byte(`{"hello":"ctr"}`)
	req := &http.HTTPRequest{Body: original}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	require.NoError(t, dp.After(decCtx))
	assert.Equal(t, original, resp.Body)
}

func TestDecryptorPluginAfterEmptyBody(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	dp := NewDecryptorPlugin(a)

	resp := &http.HTTPResponse{}
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	ctx.SetResponse(resp)
	require.NoError(t, dp.After(ctx))
}

func TestDecryptorPluginBeforeNoop(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	dp := NewDecryptorPlugin(a)
	ctx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	assert.NoError(t, dp.Before(ctx))
}

func TestDecryptorPluginAfterWrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	rand.Read(key2)

	a1, err := NewAES(key1, WithMode(ModeGCM))
	require.NoError(t, err)
	a2, err := NewAES(key2, WithMode(ModeGCM))
	require.NoError(t, err)

	ep := NewEncryptorPlugin(a1)
	dp := NewDecryptorPlugin(a2)

	req := &http.HTTPRequest{Body: []byte("test data")}
	encCtx := plugin.NewContext(context.Background(), req)
	require.NoError(t, ep.Before(encCtx))

	resp := &http.HTTPResponse{Body: req.Body}
	decCtx := plugin.NewContext(context.Background(), &http.HTTPRequest{})
	decCtx.SetResponse(resp)
	err = dp.After(decCtx)
	assert.Error(t, err)
}

func TestEncryptorPluginAlgorithmName(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	ep := NewEncryptorPlugin(a)
	assert.Equal(t, "aes-256-gcm", ep.AlgorithmName())
}

func TestDecryptorPluginAlgorithmNameCBC(t *testing.T) {
	a := newTestAES(t, ModeCBC)
	dp := NewDecryptorPlugin(a)
	assert.Equal(t, "aes-256-cbc", dp.AlgorithmName())
}

func TestDecryptorPluginAlgorithmNameCTR(t *testing.T) {
	a := newTestAES(t, ModeCTR)
	dp := NewDecryptorPlugin(a)
	assert.Equal(t, "aes-256-ctr", dp.AlgorithmName())
}

func TestCipherPluginInterfaces(t *testing.T) {
	a := newTestAES(t, ModeGCM)
	var _ plugin.Plugin = NewEncryptorPlugin(a)
	var _ plugin.Plugin = NewDecryptorPlugin(a)
}
