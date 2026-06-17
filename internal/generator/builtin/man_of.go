package builtin

import (
	"fmt"
	"math/rand"
	"strings"
)

// ManOf implements the __manOf system function.
// It accepts a slice of options and independently includes each one with 50% probability.
// Returns a comma-separated string of selected options (no spaces).
// Guarantees at least one option is always returned.
func ManOf(options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("manOf: no options provided")
	}

	var selected []string
	for _, opt := range options {
		if rand.Float64() < 0.5 {
			selected = append(selected, opt)
		}
	}

	// Guarantee at least one element
	if len(selected) == 0 {
		idx := rand.Intn(len(options))
		selected = append(selected, options[idx])
	}

	return strings.Join(selected, ","), nil
}