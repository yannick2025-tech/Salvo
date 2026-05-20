package plugin

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

type stubRequest struct {
	timeout time.Duration
}

func (r *stubRequest) GetTimeout() time.Duration { return r.timeout }

type stubResponse struct {
	statusCode int
	latency    time.Duration
	err        error
}

func (r *stubResponse) GetStatusCode() int        { return r.statusCode }
func (r *stubResponse) GetLatency() time.Duration { return r.latency }
func (r *stubResponse) GetError() error           { return r.err }

type stubPlugin struct {
	name     string
	priority int
	beforeFn func(*Context) error
	afterFn  func(*Context) error
}

func (p *stubPlugin) Name() string              { return p.name }
func (p *stubPlugin) Priority() int             { return p.priority }
func (p *stubPlugin) Before(ctx *Context) error { return p.beforeFn(ctx) }
func (p *stubPlugin) After(ctx *Context) error  { return p.afterFn(ctx) }

func TestContextNew(t *testing.T) {
	req := &stubRequest{timeout: 5 * time.Second}
	ctx := NewContext(context.Background(), req)

	assert.Equal(t, PhaseBefore, ctx.Phase())
	assert.Equal(t, req, ctx.Request())
	assert.Nil(t, ctx.Response())
	assert.False(t, ctx.Aborted())
	assert.Nil(t, ctx.AbortError())
}

func TestContextSetGet(t *testing.T) {
	ctx := NewContext(context.Background(), &stubRequest{})

	ctx.Set("key1", "value1")
	v, ok := ctx.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v)

	_, ok = ctx.Get("nonexistent")
	assert.False(t, ok)
}

func TestContextSetOverwrite(t *testing.T) {
	ctx := NewContext(context.Background(), &stubRequest{})

	ctx.Set("key", "v1")
	ctx.Set("key", "v2")
	v, ok := ctx.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "v2", v)
}

func TestContextAbort(t *testing.T) {
	ctx := NewContext(context.Background(), &stubRequest{})

	assert.False(t, ctx.Aborted())
	ctx.Abort(errors.New("stop"))
	assert.True(t, ctx.Aborted())
	assert.EqualError(t, ctx.AbortError(), "stop")
}

func TestContextSetPhase(t *testing.T) {
	ctx := NewContext(context.Background(), &stubRequest{})
	assert.Equal(t, PhaseBefore, ctx.Phase())

	ctx.SetPhase(PhaseAfter)
	assert.Equal(t, PhaseAfter, ctx.Phase())
}

func TestContextSetResponse(t *testing.T) {
	ctx := NewContext(context.Background(), &stubRequest{})
	assert.Nil(t, ctx.Response())

	resp := &stubResponse{statusCode: 200}
	ctx.SetResponse(resp)
	assert.Equal(t, resp, ctx.Response())
}

func TestContextUnderlying(t *testing.T) {
	bg := context.Background()
	ctx := NewContext(bg, &stubRequest{})
	assert.Equal(t, bg, ctx.Context())
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "test", priority: 10}

	require.NoError(t, r.Register(p))
	assert.Equal(t, p, r.Get("test"))
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	p1 := &stubPlugin{name: "dup", priority: 1}
	p2 := &stubPlugin{name: "dup", priority: 2}

	require.NoError(t, r.Register(p1))
	err := r.Register(p2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "removable", priority: 1}

	require.NoError(t, r.Register(p))
	assert.True(t, r.Unregister("removable"))
	assert.Nil(t, r.Get("removable"))
}

func TestRegistryUnregisterNotFound(t *testing.T) {
	r := NewRegistry()
	assert.False(t, r.Unregister("nonexistent"))
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	assert.Nil(t, r.Get("nonexistent"))
}

func TestRegistryListSortedByPriority(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubPlugin{name: "c", priority: 30})
	_ = r.Register(&stubPlugin{name: "a", priority: 10})
	_ = r.Register(&stubPlugin{name: "b", priority: 20})

	list := r.List()
	require.Len(t, list, 3)
	assert.Equal(t, "a", list[0].Name())
	assert.Equal(t, "b", list[1].Name())
	assert.Equal(t, "c", list[2].Name())
}

func TestRegistryListReturnsCopy(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubPlugin{name: "x", priority: 1})

	list1 := r.List()
	list2 := r.List()
	assert.NotSame(t, &list1[0], &list2[0])
}

func TestRunBeforeOrder(t *testing.T) {
	r := NewRegistry()
	var order []string

	_ = r.Register(&stubPlugin{
		name:     "second",
		priority: 20,
		beforeFn: func(ctx *Context) error { order = append(order, "second"); return nil },
	})
	_ = r.Register(&stubPlugin{
		name:     "first",
		priority: 10,
		beforeFn: func(ctx *Context) error { order = append(order, "first"); return nil },
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	require.NoError(t, r.RunBefore(ctx))
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestRunAfterReverseOrder(t *testing.T) {
	r := NewRegistry()
	var order []string

	_ = r.Register(&stubPlugin{
		name:     "second",
		priority: 20,
		afterFn:  func(ctx *Context) error { order = append(order, "second"); return nil },
	})
	_ = r.Register(&stubPlugin{
		name:     "first",
		priority: 10,
		afterFn:  func(ctx *Context) error { order = append(order, "first"); return nil },
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	require.NoError(t, r.RunAfter(ctx))
	assert.Equal(t, []string{"second", "first"}, order)
}

func TestRunBeforeAbort(t *testing.T) {
	r := NewRegistry()
	var order []string

	_ = r.Register(&stubPlugin{
		name:     "first",
		priority: 10,
		beforeFn: func(ctx *Context) error {
			order = append(order, "first")
			ctx.Abort(errors.New("halt"))
			return nil
		},
	})
	_ = r.Register(&stubPlugin{
		name:     "second",
		priority: 20,
		beforeFn: func(ctx *Context) error {
			order = append(order, "second")
			return nil
		},
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	err := r.RunBefore(ctx)
	assert.Error(t, err)
	assert.EqualError(t, err, "halt")
	assert.Equal(t, []string{"first"}, order)
}

func TestRunBeforeError(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&stubPlugin{
		name:     "fail",
		priority: 1,
		beforeFn: func(ctx *Context) error { return errors.New("boom") },
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	err := r.RunBefore(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin \"fail\" Before")
	assert.Contains(t, err.Error(), "boom")
}

func TestRunAfterError(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&stubPlugin{
		name:     "fail",
		priority: 1,
		afterFn:  func(ctx *Context) error { return errors.New("after-boom") },
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	err := r.RunAfter(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin \"fail\" After")
}

func TestPluginInterCommunication(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&stubPlugin{
		name:     "producer",
		priority: 10,
		beforeFn: func(ctx *Context) error {
			ctx.Set("token", "abc123")
			return nil
		},
	})
	_ = r.Register(&stubPlugin{
		name:     "consumer",
		priority: 20,
		beforeFn: func(ctx *Context) error {
			token, ok := ctx.Get("token")
			if !ok {
				return errors.New("token not found")
			}
			ctx.Set("verified", token == "abc123")
			return nil
		},
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	require.NoError(t, r.RunBefore(ctx))

	verified, ok := ctx.Get("verified")
	assert.True(t, ok)
	assert.True(t, verified.(bool))
}

func TestOnionModel(t *testing.T) {
	r := NewRegistry()
	var order []string

	_ = r.Register(&stubPlugin{
		name:     "outer",
		priority: 10,
		beforeFn: func(ctx *Context) error { order = append(order, "outer-before"); return nil },
		afterFn:  func(ctx *Context) error { order = append(order, "outer-after"); return nil },
	})
	_ = r.Register(&stubPlugin{
		name:     "inner",
		priority: 20,
		beforeFn: func(ctx *Context) error { order = append(order, "inner-before"); return nil },
		afterFn:  func(ctx *Context) error { order = append(order, "inner-after"); return nil },
	})

	ctx := NewContext(context.Background(), &stubRequest{})
	require.NoError(t, r.RunBefore(ctx))
	require.NoError(t, r.RunAfter(ctx))

	assert.Equal(t, []string{
		"outer-before", "inner-before",
		"inner-after", "outer-after",
	}, order)
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	for i := 0; i < 100; i++ {
		i := i
		go func() {
			_ = r.Register(&stubPlugin{
				name:     fmt.Sprintf("p-%d", i),
				priority: i,
			})
		}()
	}

	time.Sleep(100 * time.Millisecond)
	assert.GreaterOrEqual(t, len(r.List()), 1)
}

func TestPhaseConstants(t *testing.T) {
	assert.Equal(t, Phase(0), PhaseBefore)
	assert.Equal(t, Phase(1), PhaseAfter)
}

func TestPluginInterfaceCompliance(t *testing.T) {
	var _ Plugin = &stubPlugin{}
}

func TestContextImplementsProtocolRequest(t *testing.T) {
	var _ protocol.Request = &stubRequest{}
}

func TestContextImplementsProtocolResponse(t *testing.T) {
	var _ protocol.Response = &stubResponse{}
}
