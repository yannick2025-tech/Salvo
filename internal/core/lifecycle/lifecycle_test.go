package lifecycle

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLifecycleGlobalSetup(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "global_setup")
		return nil
	})

	err := lc.Run(context.Background(), HookGlobalSetup)
	require.NoError(t, err)
	assert.Equal(t, []string{"global_setup"}, executed)
}

func TestLifecycleGlobalTeardown(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookGlobalTeardown, func(ctx context.Context) error {
		executed = append(executed, "global_teardown")
		return nil
	})

	err := lc.Run(context.Background(), HookGlobalTeardown)
	require.NoError(t, err)
	assert.Equal(t, []string{"global_teardown"}, executed)
}

func TestLifecycleSceneSetup(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookSceneSetup, func(ctx context.Context) error {
		executed = append(executed, "scene_setup")
		return nil
	})

	err := lc.Run(context.Background(), HookSceneSetup)
	require.NoError(t, err)
	assert.Equal(t, []string{"scene_setup"}, executed)
}

func TestLifecycleSceneTeardown(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookSceneTeardown, func(ctx context.Context) error {
		executed = append(executed, "scene_teardown")
		return nil
	})

	err := lc.Run(context.Background(), HookSceneTeardown)
	require.NoError(t, err)
	assert.Equal(t, []string{"scene_teardown"}, executed)
}

func TestLifecycleMultipleHooksOrder(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "setup_1")
		return nil
	})
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "setup_2")
		return nil
	})

	err := lc.Run(context.Background(), HookGlobalSetup)
	require.NoError(t, err)
	assert.Equal(t, []string{"setup_1", "setup_2"}, executed)
}

func TestLifecycleTeardownRunsOnSetupError(t *testing.T) {
	var executed []string
	lc := New()
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "setup")
		return assert.AnError
	})
	lc.Register(HookGlobalTeardown, func(ctx context.Context) error {
		executed = append(executed, "teardown")
		return nil
	})

	_ = lc.Run(context.Background(), HookGlobalSetup)
	err := lc.Run(context.Background(), HookGlobalTeardown)
	require.NoError(t, err)
	assert.Contains(t, executed, "teardown")
}

func TestLifecycleHookError(t *testing.T) {
	lc := New()
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		return assert.AnError
	})

	err := lc.Run(context.Background(), HookGlobalSetup)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestLifecycleRunNoHooks(t *testing.T) {
	lc := New()
	err := lc.Run(context.Background(), HookGlobalSetup)
	assert.NoError(t, err)
}

func TestLifecycleContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lc := New()
	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		return ctx.Err()
	})

	err := lc.Run(ctx, HookGlobalSetup)
	assert.Error(t, err)
}

func TestLifecycleFullScenario(t *testing.T) {
	var executed []string
	lc := New()

	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "global_setup")
		return nil
	})
	lc.Register(HookSceneSetup, func(ctx context.Context) error {
		executed = append(executed, "scene_setup")
		return nil
	})
	lc.Register(HookSceneTeardown, func(ctx context.Context) error {
		executed = append(executed, "scene_teardown")
		return nil
	})
	lc.Register(HookGlobalTeardown, func(ctx context.Context) error {
		executed = append(executed, "global_teardown")
		return nil
	})

	_ = lc.Run(context.Background(), HookGlobalSetup)
	_ = lc.Run(context.Background(), HookSceneSetup)
	_ = lc.Run(context.Background(), HookSceneTeardown)
	_ = lc.Run(context.Background(), HookGlobalTeardown)

	assert.Equal(t, []string{
		"global_setup",
		"scene_setup",
		"scene_teardown",
		"global_teardown",
	}, executed)
}

func TestLifecycleClear(t *testing.T) {
	var executed []string
	lc := New()

	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "setup")
		return nil
	})

	lc.Clear(HookGlobalSetup)
	err := lc.Run(context.Background(), HookGlobalSetup)
	require.NoError(t, err)
	assert.Empty(t, executed)
}

func TestLifecycleClearAll(t *testing.T) {
	var executed []string
	lc := New()

	lc.Register(HookGlobalSetup, func(ctx context.Context) error {
		executed = append(executed, "global_setup")
		return nil
	})
	lc.Register(HookSceneSetup, func(ctx context.Context) error {
		executed = append(executed, "scene_setup")
		return nil
	})

	lc.ClearAll()
	_ = lc.Run(context.Background(), HookGlobalSetup)
	_ = lc.Run(context.Background(), HookSceneSetup)
	assert.Empty(t, executed)
}
