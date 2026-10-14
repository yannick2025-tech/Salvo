package builtin

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneOf(t *testing.T) {
	t.Parallel()

	t.Run("three options equal probability", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := OneOf([]string{"A", "B", "C"})
			require.NoError(t, err)
			counts[res]++
		}
		assert.InDelta(t, 1.0/3.0, float64(counts["A"])/samples, tolerance)
		assert.InDelta(t, 1.0/3.0, float64(counts["B"])/samples, tolerance)
		assert.InDelta(t, 1.0/3.0, float64(counts["C"])/samples, tolerance)
	})

	t.Run("single option", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := OneOf([]string{"X"})
			require.NoError(t, err)
			assert.Equal(t, "X", res)
		}
	})

	t.Run("two options equal probability", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := OneOf([]string{"A", "B"})
			require.NoError(t, err)
			counts[res]++
		}
		assert.InDelta(t, 0.5, float64(counts["A"])/samples, tolerance)
		assert.InDelta(t, 0.5, float64(counts["B"])/samples, tolerance)
	})

	t.Run("empty args", func(t *testing.T) {
		_, err := OneOf(nil)
		require.Error(t, err)
		_, err = OneOf([]string{})
		require.Error(t, err)
	})

	t.Run("special characters", func(t *testing.T) {
		res, err := OneOf([]string{"hello world", "foo,bar"})
		require.NoError(t, err)
		assert.Contains(t, []string{"hello world", "foo,bar"}, res)
	})

	t.Run("concurrent safety", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := OneOf([]string{"A", "B", "C"})
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
	})
}

func TestOneOfLargeOptions(t *testing.T) {
	t.Parallel()
	// 100 options, each should appear approximately equally
	const samples = 20000
	const optCount = 100
	opts := make([]string, optCount)
	for i := range optCount {
		opts[i] = fmt.Sprintf("opt%d", i)
	}

	counts := make(map[string]int)
	for i := 0; i < samples; i++ {
		res, err := OneOf(opts)
		require.NoError(t, err)
		counts[res]++
	}

	// Each option expected: 20000/100 = 200. Allow 3σ deviation: √(20000*0.01*0.99) ≈ 14, so 3σ ≈ 42
	for _, opt := range opts {
		assert.InDelta(t, samples/optCount, counts[opt], 50, "option %s count unexpected", opt)
	}
}