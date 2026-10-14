package builtin

import (
	"fmt"
	"math/rand"
)

// OneOf implements the __oneOf system function.
// It accepts a slice of options and returns one at random with equal probability.
func OneOf(options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("oneOf: no options provided")
	}

	idx := rand.Intn(len(options))
	return options[idx], nil
}