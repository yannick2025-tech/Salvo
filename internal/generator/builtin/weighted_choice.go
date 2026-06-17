package builtin

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// WeightedChoice implements the __weightedChoice system function.
// It accepts a single argument in the format "option1=weight1,option2=weight2,..."
// and returns one option based on weighted random selection.
// Weights are normalized to sum to 1.0 for selection probability.
// Zero and negative weights are filtered out before selection.
func WeightedChoice(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("weightedChoice: empty input")
	}

	pairs := strings.Split(input, ",")
	if len(pairs) == 0 {
		return "", fmt.Errorf("weightedChoice: no options")
	}

	type option struct {
		key    string
		weight float64
	}

	var options []option
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Each pair must be "key=weight".
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			return "", fmt.Errorf("weightedChoice: malformed option %q (missing '=')", pair)
		}

		// Multiple '=' signs are not allowed.
		if strings.Count(pair, "=") > 1 {
			return "", fmt.Errorf("weightedChoice: malformed option %q (multiple '=')", pair)
		}

		key := strings.TrimSpace(pair[:eqIdx])
		weightStr := strings.TrimSpace(pair[eqIdx+1:])

		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil {
			return "", fmt.Errorf("weightedChoice: non-numeric weight %q in option %q", weightStr, pair)
		}

		if weight > 0 {
			options = append(options, option{key: key, weight: weight})
		}
	}

	if len(options) == 0 {
		return "", fmt.Errorf("weightedChoice: no valid (positive weight) options")
	}

	// Normalize weights to cumulative distribution.
	var totalWeight float64
	for _, opt := range options {
		totalWeight += opt.weight
	}

	// Select based on random value.
	r := rand.Float64() * totalWeight
	var cumulative float64
	for _, opt := range options {
		cumulative += opt.weight
		if r < cumulative {
			return opt.key, nil
		}
	}

	// Fallback (should not reach here due to floating point).
	return options[len(options)-1].key, nil
}
