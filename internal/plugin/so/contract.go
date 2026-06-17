// Package so provides a dynamic plugin loading system for Salvo.
//
// SO plugins are compiled Go shared libraries (.so files) that export
// a New function returning a Plugin interface. Once loaded, they can
// be called from expression engine expressions using the __so() function.
package so

// Plugin is the interface that every SO plugin must implement.
// It provides versioned operations callable from expressions.
type Plugin interface {
	// Name returns the plugin name (e.g. "shell-aes").
	Name() string

	// Version returns the plugin version (e.g. "1.0.0").
	Version() string

	// Call executes a named operation with the given arguments.
	// Returns the result as a string suitable for expression substitution.
	Call(op string, args []string) (string, error)
}

// Factory is the signature that every .so plugin must export as "New".
// When a .so file is loaded, the Loader calls plugin.New() to create
// a Plugin instance. If Init returns an error, the plugin is discarded.
type Factory func() (Plugin, error)