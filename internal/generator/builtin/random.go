package builtin

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
)

// Random implements the __random system function.
// Two modes:
//   - Integer: Random([]string{min, max}) → random integer in [min, max]
//   - Float:   Random([]string{min, max, scale}) → random float with `scale` decimal places
// If min > max, returns min as a fallback.
func Random(args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", fmt.Errorf("random: expected 2 or 3 arguments, got %d", len(args))
	}

	minVal, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return "", fmt.Errorf("random: invalid min argument %q", args[0])
	}

	maxVal, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return "", fmt.Errorf("random: invalid max argument %q", args[1])
	}

	if minVal > maxVal {
		// When min > max, return the min value as a fallback.
		if len(args) == 3 {
			scale, err := strconv.Atoi(args[2])
			if err != nil {
				return "", fmt.Errorf("random: invalid scale argument %q", args[2])
			}
			format := fmt.Sprintf("%%.%df", scale)
			return fmt.Sprintf(format, maxVal), nil // maxVal is the smaller value
		}
		return fmt.Sprintf("%d", int64(maxVal)), nil // maxVal is the smaller value
	}

	if len(args) == 3 {
		// Float mode
		scale, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("random: invalid scale argument %q", args[2])
		}
		r := minVal + rand.Float64()*(maxVal-minVal)
		format := fmt.Sprintf("%%.%df", scale)
		return fmt.Sprintf(format, r), nil
	}

	// Integer mode
	minInt := int64(math.Ceil(minVal))
	maxInt := int64(math.Floor(maxVal))
	if minInt > maxInt {
		return fmt.Sprintf("%d", minInt), nil
	}
	r := minInt + int64(rand.Float64()*float64(maxInt-minInt+1))
	return fmt.Sprintf("%d", r), nil
}