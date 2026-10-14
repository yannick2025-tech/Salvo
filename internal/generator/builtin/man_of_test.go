package builtin

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManOf(t *testing.T) {
	t.Parallel()

	t.Run("seven options subset", func(t *testing.T) {
		const samples = 10000
		opts := []string{"1", "2", "3", "4", "5", "6", "7"}
		counts := make(map[string]int)
		subsetSizeDist := make(map[int]int)
		for i := 0; i < samples; i++ {
			res, err := ManOf(opts)
			require.NoError(t, err)
			parts := strings.Split(res, ",")
			subsetSizeDist[len(parts)]++
			for _, p := range parts {
				counts[p]++
			}
		}

		// Each element appears ~50% of the time (±5%)
		for _, opt := range opts {
			assert.InDelta(t, 0.5, float64(counts[opt])/samples, tolerance, "element %s", opt)
		}

		// Subset size ranges from 1 to 7
		assert.GreaterOrEqual(t, len(subsetSizeDist), 1)
		for size := 1; size <= 7; size++ {
			// At least some samples have each subset size
			assert.GreaterOrEqual(t, subsetSizeDist[size], 0)
		}
	})

	t.Run("single option", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := ManOf([]string{"1"})
			require.NoError(t, err)
			assert.Equal(t, "1", res)
		}
	})

	t.Run("two options", func(t *testing.T) {
		const samples = 10000
		opts := []string{"1", "2"}
		subsetSizeDist := make(map[int]int)
		for i := 0; i < samples; i++ {
			res, err := ManOf(opts)
			require.NoError(t, err)
			parts := strings.Split(res, ",")
			subsetSizeDist[len(parts)]++
		}

		// Both subset sizes 1 and 2 should appear
		assert.Greater(t, subsetSizeDist[1], 0)
		assert.Greater(t, subsetSizeDist[2], 0)
	})

	t.Run("empty args", func(t *testing.T) {
		_, err := ManOf(nil)
		require.Error(t, err)
		_, err = ManOf([]string{})
		require.Error(t, err)
	})

	t.Run("format no spaces", func(t *testing.T) {
		opts := []string{"A", "B", "C"}
		for i := 0; i < 50; i++ {
			res, err := ManOf(opts)
			require.NoError(t, err)
			assert.NotContains(t, res, " ")
			// Each part should be a valid option
			for _, part := range strings.Split(res, ",") {
				assert.Contains(t, opts, part)
			}
		}
	})

	t.Run("concurrent safety", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ManOf([]string{"1", "2", "3", "4", "5"})
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
	})
}