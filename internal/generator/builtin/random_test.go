package builtin

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandom(t *testing.T) {
	t.Parallel()

	t.Run("integer range", func(t *testing.T) {
		const samples = 10000
		var sum int
		for i := 0; i < samples; i++ {
			res, err := Random([]string{"60", "600"})
			require.NoError(t, err)
			val, err := strconv.Atoi(res)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, val, 60)
			assert.LessOrEqual(t, val, 600)
			sum += val
		}
		avg := float64(sum) / samples
		assert.InDelta(t, 330.0, avg, 30.0)
	})

	t.Run("integer min equals max", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := Random([]string{"1", "1"})
			require.NoError(t, err)
			assert.Equal(t, "1", res)
		}
	})

	t.Run("integer range zero to hundred", func(t *testing.T) {
		const samples = 2000
		for i := 0; i < samples; i++ {
			res, err := Random([]string{"0", "100"})
			require.NoError(t, err)
			val, err := strconv.Atoi(res)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, val, 0)
			assert.LessOrEqual(t, val, 100)
		}
	})

	t.Run("integer min greater than max", func(t *testing.T) {
		res, err := Random([]string{"600", "60"})
		require.NoError(t, err)
		assert.Equal(t, "60", res)
	})

	t.Run("float range with scale", func(t *testing.T) {
		const samples = 500
		for i := 0; i < samples; i++ {
			res, err := Random([]string{"1.5", "9.5", "2"})
			require.NoError(t, err)
			val, err := strconv.ParseFloat(res, 64)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, val, 1.5)
			assert.LessOrEqual(t, val, 9.5)
			// Verify 2 decimal places
			parts := strings.Split(res, ".")
			require.Len(t, parts, 2)
			assert.Len(t, parts[1], 2)
		}
	})

	t.Run("float scale zero", func(t *testing.T) {
		res, err := Random([]string{"1.0", "10.0", "0"})
		require.NoError(t, err)
		// Scale 0 → no decimal point
		assert.NotContains(t, res, ".")
		val, err := strconv.Atoi(res)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, val, 1)
		assert.LessOrEqual(t, val, 10)
	})

	t.Run("float scale four", func(t *testing.T) {
		res, err := Random([]string{"0.0", "1.0", "4"})
		require.NoError(t, err)
		parts := strings.Split(res, ".")
		require.Len(t, parts, 2)
		assert.Len(t, parts[1], 4)
		val, err := strconv.ParseFloat(res, 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, val, 0.0)
		assert.LessOrEqual(t, val, 1.0)
	})

	t.Run("float min greater than max", func(t *testing.T) {
		res, err := Random([]string{"9.5", "1.5", "2"})
		require.NoError(t, err)
		assert.Equal(t, "1.50", res)
	})

	t.Run("non-numeric args", func(t *testing.T) {
		_, err := Random([]string{"abc", "def"})
		require.Error(t, err)
	})

	t.Run("wrong arg count - one arg", func(t *testing.T) {
		_, err := Random([]string{"60"})
		require.Error(t, err)
	})

	t.Run("wrong arg count - four args", func(t *testing.T) {
		_, err := Random([]string{"60", "600", "2", "3"})
		require.Error(t, err)
	})

	t.Run("concurrent safety", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := Random([]string{"60", "600"})
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
	})
}

func TestRandomDistribution(t *testing.T) {
	t.Parallel()
	const samples = 10000
	// Test that float random values are uniformly distributed
	buckets := make([]int, 10)
	for i := 0; i < samples; i++ {
		res, err := Random([]string{"0.0", "10.0", "1"})
		require.NoError(t, err)
		val, err := strconv.ParseFloat(res, 64)
		require.NoError(t, err)
		bucket := int(math.Floor(val))
		if bucket == 10 {
			bucket = 9
		}
		buckets[bucket]++
	}
	// Each bucket should be roughly 1000 ± 200
	for i, count := range buckets {
		assert.InDelta(t, samples/10, count, 200, "bucket %d", i)
	}
}

func TestRandomFloatScaleEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("scale 6", func(t *testing.T) {
		res, err := Random([]string{"0", "1", "6"})
		require.NoError(t, err)
		parts := strings.Split(res, ".")
		require.Len(t, parts, 2)
		assert.Len(t, parts[1], 6)
	})

	t.Run("large range integer", func(t *testing.T) {
		res, err := Random([]string{"0", "1000000"})
		require.NoError(t, err)
		val, err := strconv.Atoi(res)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, val, 0)
		assert.LessOrEqual(t, val, 1000000)
	})
}