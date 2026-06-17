package builtin

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnowflakeId(t *testing.T) {
	t.Parallel()

	t.Run("format numeric string", func(t *testing.T) {
		res, err := SnowflakeId()
		require.NoError(t, err)
		// IDs are 17-19 decimal digits (64-bit integer)
		assert.GreaterOrEqual(t, len(res), 17)
		assert.LessOrEqual(t, len(res), 19)
		_, err = strconv.ParseInt(res, 10, 64)
		require.NoError(t, err, "must be a valid integer")
	})

	t.Run("uniqueness 10000 calls", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 10000; i++ {
			res, err := SnowflakeId()
			require.NoError(t, err)
			assert.False(t, ids[res], "duplicate ID: %s", res)
			ids[res] = true
		}
		assert.Equal(t, 10000, len(ids))
	})

	t.Run("monotonically increasing", func(t *testing.T) {
		prev := ""
		for i := 0; i < 100; i++ {
			cur, err := SnowflakeId()
			require.NoError(t, err)
			if prev != "" {
				assert.Greater(t, cur, prev, "ID must be monotonically increasing")
			}
			prev = cur
		}
	})

	t.Run("concurrent safety", func(t *testing.T) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					res, err := SnowflakeId()
					assert.NoError(t, err)
					mu.Lock()
					assert.False(t, ids[res], "duplicate ID: %s", res)
					ids[res] = true
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 10000, len(ids))
	})
}