package builtin

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tolerance accepts probability deviation up to 5% for 10000 samples.
const tolerance = 0.05

func TestWeightedChoice(t *testing.T) {
	t.Parallel()

	t.Run("binary equal weight", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := WeightedChoice("1=50,0=50")
			require.NoError(t, err)
			counts[res]++
		}
		assert.InDelta(t, 0.5, float64(counts["1"])/samples, tolerance)
		assert.InDelta(t, 0.5, float64(counts["0"])/samples, tolerance)
	})

	t.Run("multi weighted", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := WeightedChoice("A=40,B=30,C=20,D=10")
			require.NoError(t, err)
			counts[res]++
		}
		assert.InDelta(t, 0.4, float64(counts["A"])/samples, tolerance)
		assert.InDelta(t, 0.3, float64(counts["B"])/samples, tolerance)
		assert.InDelta(t, 0.2, float64(counts["C"])/samples, tolerance)
		assert.InDelta(t, 0.1, float64(counts["D"])/samples, tolerance)
	})

	t.Run("weight sum < 100 normalization", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := WeightedChoice("A=40,B=30,C=20")
			require.NoError(t, err)
			counts[res]++
		}
		// Sum = 90, expected: A≈44.4%, B≈33.3%, C≈22.2%
		assert.InDelta(t, 40.0/90.0, float64(counts["A"])/samples, tolerance)
		assert.InDelta(t, 30.0/90.0, float64(counts["B"])/samples, tolerance)
		assert.InDelta(t, 20.0/90.0, float64(counts["C"])/samples, tolerance)
	})

	t.Run("weight sum > 100 normalization", func(t *testing.T) {
		const samples = 10000
		counts := make(map[string]int)
		for i := 0; i < samples; i++ {
			res, err := WeightedChoice("A=50,B=50,C=50")
			require.NoError(t, err)
			counts[res]++
		}
		// Sum = 150, each ≈33.3%
		assert.InDelta(t, 1.0/3.0, float64(counts["A"])/samples, tolerance)
		assert.InDelta(t, 1.0/3.0, float64(counts["B"])/samples, tolerance)
		assert.InDelta(t, 1.0/3.0, float64(counts["C"])/samples, tolerance)
	})

	t.Run("single option", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := WeightedChoice("A=100")
			require.NoError(t, err)
			assert.Equal(t, "A", res)
		}
	})

	t.Run("zero weight filtered", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := WeightedChoice("A=0,B=50")
			require.NoError(t, err)
			assert.Equal(t, "B", res)
		}
	})

	t.Run("negative weight filtered", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			res, err := WeightedChoice("A=-1,B=50")
			require.NoError(t, err)
			assert.Equal(t, "B", res)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := WeightedChoice("")
		require.Error(t, err)
	})

	t.Run("malformed no equals", func(t *testing.T) {
		_, err := WeightedChoice("A,B,C")
		require.Error(t, err)
	})

	t.Run("non-numeric weight", func(t *testing.T) {
		_, err := WeightedChoice("A=xx")
		require.Error(t, err)
	})

	t.Run("multiple equals signs", func(t *testing.T) {
		_, err := WeightedChoice("A=50=extra")
		require.Error(t, err)
	})

	t.Run("concurrent safety", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := WeightedChoice("A=50,B=50")
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
	})
}

// --- Helper: check that WeightedChoice doesn't generate all-zero probability
// when only zero/negative weights exist ---

func TestWeightedChoiceAllZeroWeights(t *testing.T) {
	t.Parallel()
	_, err := WeightedChoice("A=0,B=0")
	require.Error(t, err)
}

func TestWeightedChoiceAllNegativeWeights(t *testing.T) {
	t.Parallel()
	_, err := WeightedChoice("A=-1,B=-2")
	require.Error(t, err)
}

func TestWeightedChoiceLargeSampleVerifyDistribution(t *testing.T) {
	t.Parallel()
	// Use a 2:1 ratio (≈66.7% vs ≈33.3%) with 20000 samples
	const samples = 20000
	counts := make(map[string]int)
	for i := 0; i < samples; i++ {
		res, err := WeightedChoice("X=200,Y=100")
		require.NoError(t, err)
		counts[res]++
	}
	ratio := float64(counts["X"]) / float64(counts["Y"])
	assert.InDelta(t, 2.0, ratio, 0.2) // Allow wider tolerance for ratio

	// Check that both keys appear
	assert.Greater(t, counts["X"], 0)
	assert.Greater(t, counts["Y"], 0)
}

// TestWeightedChoiceDeterministicEdgeCases covers edge cases that should
// always produce the same result regardless of random seed.
func TestWeightedChoiceDeterministicEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "single option 100", input: "only=100", want: "only"},
		{name: "only positive after filtering", input: "zero=0,only=100", want: "only"},
		{name: "option with special chars", input: "hello world=50,foo@bar=50", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WeightedChoice(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.want != "" {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestWeightedChoiceExactCounts runs a smaller sample where we verify
// exact count bounds using binomial confidence (practical certainty).
func TestWeightedChoiceTwoOptionExactCounts(t *testing.T) {
	t.Parallel()
	const samples = 10000
	countA := 0
	for i := 0; i < samples; i++ {
		res, err := WeightedChoice("A=70,B=30")
		require.NoError(t, err)
		if res == "A" {
			countA++
		}
	}
	// With 10000 samples and p=0.7, 99.9% CI is approximately [6850, 7150]
	assert.InDelta(t, 7000, countA, 200)
}

func TestWeightedChoiceWeightsMustBePositive(t *testing.T) {
	t.Parallel()
	_, err := WeightedChoice("A=-0.5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid")
}

// BenchmarkWeightedChoice measures performance of weighted choice.
func BenchmarkWeightedChoice(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = WeightedChoice("A=30,B=25,C=20,D=15,E=10")
	}
}

// --- Statistical distribution helper for probability functions ---

func assertProbability(t *testing.T, fn func(string) (string, error), input string, expected map[string]float64, samples int) {
	t.Helper()
	counts := make(map[string]int)
	for i := 0; i < samples; i++ {
		res, err := fn(input)
		require.NoError(t, err)
		counts[res]++
	}

	for key, expProb := range expected {
		actualProb := float64(counts[key]) / float64(samples)
		// Use a tolerance that scales with sample size
		adjTolerance := tolerance + 1.0/math.Sqrt(float64(samples))
		assert.InDelta(t, expProb, actualProb, adjTolerance, "key=%s expected=%.4f actual=%.4f", key, expProb, actualProb)
	}
}
