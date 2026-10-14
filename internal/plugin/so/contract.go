// Package so provides a dynamic plugin loading system for Salvo.
//
// SO plugins are compiled Go shared libraries (.so files) that export
// a New function returning a Plugin interface. Once loaded, they can
// be called from expression engine expressions using the __so() function.
//
// The Plugin and Factory types are defined in the contract sub-package
// to ensure binary compatibility between plugins and the main binary.
package so

import "github.com/yannick2025-tech/Salvo/internal/plugin/so/contract"

// Plugin is the interface that every SO plugin must implement.
// It provides versioned operations callable from expressions.
type Plugin = contract.Plugin

// Factory is the signature that every .so plugin must export as "New".
// When a .so file is loaded, the Loader calls plugin.New() to create
// a Plugin instance. If Init returns an error, the plugin is discarded.
type Factory = contract.Factory