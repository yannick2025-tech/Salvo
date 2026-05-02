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

func TestLimiterName(t *testing.T) {
	l := NewLimiter(10, 5)
	assert.Equal(t, "ratelimiter", l.Name())
}

func TestLimiterCustomName(t *testing.T) {
	l := NewLimiter(10, 5, WithName("my-limiter"))
	assert.Equal(t, "my-limiter", l.Name())
}

func TestLimiterPriority(t *testing.T) {
	l := NewLimiter(10, 5)
	assert.Equal(t, 1, l.Priority())
}

func TestLimiterCustomPriority(t *testing.T) {
	l := NewLimiter(10, 5, WithPriority(50))
	assert.Equal(t, 50, l.Priority())
}

func TestLimiterRate(t *testing.T) {
	l := NewLimiter(100, 10)
	assert.Equal(t, 100.0, l.Rate())
}

func TestLimiterBurst(t *testing.T) {
	l := NewLimiter(100, 10)
	assert.Equal(t, 10, l.Burst())
}

func TestLimiterAllowsUpToBurst(t *testing.T) {
	l := NewLimiter(0, 3)

	for i := 0; i < 3; i++ {
		ctx := plugin.NewContext(context.Background(), &stubReq{})
		require.NoError(t, l.Before(ctx))
		assert.False(t, ctx.Aborted())
	}
}

func TestLimiterRejectsAfterBurst(t *testing.T) {
	l := NewLimiter(0, 2)

	for i := 0; i < 2; i++ {
		ctx := plugin.NewContext(context.Background(), &stubReq{})
		require.NoError(t, l.Before(ctx))
	}

	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx))
	assert.True(t, ctx.Aborted())
	assert.Contains(t, ctx.AbortError().Error(), "rate limit exceeded")
}

func TestLimiterRefillsOverTime(t *testing.T) {
	l := NewLimiter(1000, 1)

	ctx1 := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx1))
	assert.False(t, ctx1.Aborted())

	ctx2 := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx2))
	assert.True(t, ctx2.Aborted())

	time.Sleep(20 * time.Millisecond)

	ctx3 := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx3))
	assert.False(t, ctx3.Aborted())
}

func TestLimiterAfterIsNoop(t *testing.T) {
	l := NewLimiter(10, 5)
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	assert.NoError(t, l.After(ctx))
}

func TestLimiterTokens(t *testing.T) {
	l := NewLimiter(0, 5)
	assert.Equal(t, 5.0, l.Tokens())
}

func TestLimiterTokensAfterAcquire(t *testing.T) {
	l := NewLimiter(0, 5)
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx))
	assert.Equal(t, 4.0, l.Tokens())
}

func TestLimiterTokensCappedAtBurst(t *testing.T) {
	l := NewLimiter(1e6, 3)
	time.Sleep(10 * time.Millisecond)
	tokens := l.Tokens()
	assert.LessOrEqual(t, tokens, 3.0)
}

func TestLimiterWithRegistry(t *testing.T) {
	r := plugin.NewRegistry()
	l := NewLimiter(100, 5)
	require.NoError(t, r.Register(l))

	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, r.RunBefore(ctx))
	assert.False(t, ctx.Aborted())
}

func TestLimiterPluginInterface(t *testing.T) {
	var _ plugin.Plugin = NewLimiter(10, 5)
}

func TestLimiterZeroRateZeroBurst(t *testing.T) {
	l := NewLimiter(0, 0)
	ctx := plugin.NewContext(context.Background(), &stubReq{})
	require.NoError(t, l.Before(ctx))
	assert.True(t, ctx.Aborted())
}
