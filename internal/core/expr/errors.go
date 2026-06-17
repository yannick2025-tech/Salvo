package expr

import "fmt"

// ErrDuplicateRegistration is returned when a function name is already registered.
type ErrDuplicateRegistration struct {
	Name string
}

func (e *ErrDuplicateRegistration) Error() string {
	return fmt.Sprintf("function %q already registered", e.Name)
}