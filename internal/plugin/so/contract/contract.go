// Package contract defines the Plugin interface and Factory type for SO plugins.
// This package has zero external dependencies so that plugins can import it
// without pulling in the entire application dependency graph, ensuring
// binary compatibility between .so plugins and the main binary.
package contract

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
// a Plugin instance. If it returns an error, the plugin is discarded.
type Factory func() (Plugin, error)
