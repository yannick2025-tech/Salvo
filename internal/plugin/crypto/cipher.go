// Package crypto provides encryption, decryption, and hashing plugins
// for Salvo. It defines two independent abstraction hierarchies:
//
//   - Encryptor / Decryptor: symmetric cipher operations (AES-GCM, etc.)
//   - Hasher / Verifier: integrity and signing operations (HMAC, etc.)
//
// Each algorithm implements the relevant interface and is wrapped by a
// plugin adapter (EncryptorPlugin, DecryptorPlugin, HasherPlugin,
// VerifierPlugin) for integration with the plugin.Registry.
package crypto

import (
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// Encryptor is the interface for symmetric encryption algorithms.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns ciphertext.
	Encrypt(plaintext []byte) ([]byte, error)
	// Algorithm returns the algorithm identifier (e.g. "aes-256-gcm").
	Algorithm() string
}

// Decryptor is the interface for symmetric decryption algorithms.
type Decryptor interface {
	// Decrypt decrypts ciphertext and returns plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)
	// Algorithm returns the algorithm identifier (e.g. "aes-256-gcm").
	Algorithm() string
}

// EncryptorPlugin adapts an Encryptor into a plugin.Plugin that
// encrypts the request body in the Before phase.
type EncryptorPlugin struct {
	encryptor  Encryptor
	priority   int
	pluginName string
}

// EncryptorPluginOption configures an EncryptorPlugin.
type EncryptorPluginOption func(*EncryptorPlugin)

// WithEncryptorPriority sets the plugin priority.
func WithEncryptorPriority(p int) EncryptorPluginOption {
	return func(ep *EncryptorPlugin) { ep.priority = p }
}

// WithEncryptorPluginName sets a custom plugin name.
func WithEncryptorPluginName(n string) EncryptorPluginOption {
	return func(ep *EncryptorPlugin) { ep.pluginName = n }
}

// NewEncryptorPlugin wraps an Encryptor as a plugin.Plugin.
func NewEncryptorPlugin(e Encryptor, opts ...EncryptorPluginOption) *EncryptorPlugin {
	ep := &EncryptorPlugin{
		encryptor:  e,
		priority:   5,
		pluginName: e.Algorithm() + "-encryptor",
	}
	for _, opt := range opts {
		opt(ep)
	}
	return ep
}

// Name implements plugin.Plugin.
func (ep *EncryptorPlugin) Name() string { return ep.pluginName }

// Priority implements plugin.Plugin.
func (ep *EncryptorPlugin) Priority() int { return ep.priority }

// Before encrypts the request body if it is an HTTP request.
func (ep *EncryptorPlugin) Before(ctx *plugin.Context) error {
	req, ok := ctx.Request().(*http.HTTPRequest)
	if !ok {
		return nil
	}
	if len(req.Body) == 0 {
		return nil
	}

	encrypted, err := ep.encryptor.Encrypt(req.Body)
	if err != nil {
		return fmt.Errorf("crypto: encrypt: %w", err)
	}
	req.Body = encrypted
	return nil
}

// After is a no-op for the encryptor.
func (ep *EncryptorPlugin) After(_ *plugin.Context) error { return nil }

// AlgorithmName returns the underlying algorithm name.
func (ep *EncryptorPlugin) AlgorithmName() string { return ep.encryptor.Algorithm() }

// DecryptorPlugin adapts a Decryptor into a plugin.Plugin that
// decrypts the response body in the After phase.
type DecryptorPlugin struct {
	decryptor  Decryptor
	priority   int
	pluginName string
}

// DecryptorPluginOption configures a DecryptorPlugin.
type DecryptorPluginOption func(*DecryptorPlugin)

// WithDecryptorPriority sets the plugin priority.
func WithDecryptorPriority(p int) DecryptorPluginOption {
	return func(dp *DecryptorPlugin) { dp.priority = p }
}

// WithDecryptorPluginName sets a custom plugin name.
func WithDecryptorPluginName(n string) DecryptorPluginOption {
	return func(dp *DecryptorPlugin) { dp.pluginName = n }
}

// NewDecryptorPlugin wraps a Decryptor as a plugin.Plugin.
func NewDecryptorPlugin(d Decryptor, opts ...DecryptorPluginOption) *DecryptorPlugin {
	dp := &DecryptorPlugin{
		decryptor:  d,
		priority:   95,
		pluginName: d.Algorithm() + "-decryptor",
	}
	for _, opt := range opts {
		opt(dp)
	}
	return dp
}

// Name implements plugin.Plugin.
func (dp *DecryptorPlugin) Name() string { return dp.pluginName }

// Priority implements plugin.Plugin.
func (dp *DecryptorPlugin) Priority() int { return dp.priority }

// Before is a no-op for the decryptor.
func (dp *DecryptorPlugin) Before(_ *plugin.Context) error { return nil }

// After decrypts the response body if it is an HTTP response.
func (dp *DecryptorPlugin) After(ctx *plugin.Context) error {
	resp, ok := ctx.Response().(*http.HTTPResponse)
	if !ok {
		return nil
	}
	if len(resp.Body) == 0 {
		return nil
	}

	decrypted, err := dp.decryptor.Decrypt(resp.Body)
	if err != nil {
		return fmt.Errorf("crypto: decrypt: %w", err)
	}
	resp.Body = decrypted
	return nil
}

// AlgorithmName returns the underlying algorithm name.
func (dp *DecryptorPlugin) AlgorithmName() string { return dp.decryptor.Algorithm() }

// compile-time checks
var (
	_ plugin.Plugin = (*EncryptorPlugin)(nil)
	_ plugin.Plugin = (*DecryptorPlugin)(nil)
)
