package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
)

type stubReq struct{}

func (r *stubReq) GetTimeout() time.Duration { return 0 }

func TestLimiterPluginName(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5))
	assert.Equal(t, "token-bucket", lp.Name())
}

func TestLimiterPluginCustomName(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5), WithLimiterPluginName("my-limiter"))
	assert.Equal(t, "my-limiter", lp.Name())
}

func TestLimiterPluginPriority(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5))
	assert.Equal(t, 1, lp.Priority())
}

func TestLimiterPluginCustomPriority(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5), WithLimiterPriority(50))
	assert.Equal(t, 50, lp.Priority())
}

func TestLimiterPluginBeforeAllow(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5))
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, lp.Before(ctx))
	assert.False(t, ctx.Aborted())
}

func TestLimiterPluginBeforeReject(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(0, 0))
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, lp.Before(ctx))
	assert.True(t, ctx.Aborted())
	assert.Contains(t, ctx.AbortError().Error(), "rate limit exceeded")
}

func TestLimiterPluginAfterNoop(t *testing.T) {
	lp := NewLimiterPlugin(NewTokenBucket(10, 5))
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	assert.NoError(t, lp.After(ctx))
}

func TestLimiterPluginAlgorithmName(t *testing.T) {
	lp := NewLimiterPlugin(NewLeakyBucket(10, 5))
	assert.Equal(t, "leaky-bucket", lp.AlgorithmName())
}

func TestLimiterPluginLimiter(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	lp := NewLimiterPlugin(tb)
	assert.Equal(t, tb, lp.Limiter())
}

func TestLimiterPluginWithRegistry(t *testing.T) {
	r := plugin.NewRegistry()
	lp := NewLimiterPlugin(NewTokenBucket(100, 5))
	require.NoError(t, r.Register(lp))

	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, r.RunBefore(ctx))
	assert.False(t, ctx.Aborted())
}

func TestLimiterPluginInterface(t *testing.T) {
	var _ plugin.Plugin = NewLimiterPlugin(NewTokenBucket(10, 5))
}

func TestLimiterInterface(t *testing.T) {
	var _ Limiter = NewTokenBucket(10, 5)
	var _ Limiter = NewLeakyBucket(10, 5)
	var _ Limiter = NewSlidingWindow(10, time.Second)
	var _ Limiter = NewFixedWindow(10, time.Second)
}
