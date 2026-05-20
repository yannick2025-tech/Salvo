package generator

import "errors"

var (
	ErrNoGenerator  = errors.New("generator: no registered generator can handle this schema")
	ErrInvalidRange = errors.New("generator: invalid min/max range")
	ErrEmptyEnum    = errors.New("generator: enum is empty")
)
