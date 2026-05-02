package crypto

import (
	"errors"
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// Hasher is the interface for hashing/signing algorithms. It produces
// a deterministic digest from input data.
type Hasher interface {
	// Hash computes the digest of data and returns it as a hex string.
	Hash(data []byte) string
	// Algorithm returns the algorithm identifier (e.g. "hmac-sha256").
	Algorithm() string
}

// Verifier is the interface for verifying a previously computed hash.
type Verifier interface {
	// Verify checks whether the expected hex-encoded digest matches the
	// computed hash of data.
	Verify(data []byte, expectedHex string) bool
	// Algorithm returns the algorithm identifier (e.g. "hmac-sha256").
	Algorithm() string
}

// HasherPlugin adapts a Hasher into a plugin.Plugin that signs the
// request body and adds the digest as a header in the Before phase.
type HasherPlugin struct {
	hasher     Hasher
	priority   int
	pluginName string
	headerName string
}

// HasherPluginOption configures a HasherPlugin.
type HasherPluginOption func(*HasherPlugin)

// WithHasherPriority sets the plugin priority.
func WithHasherPriority(p int) HasherPluginOption {
	return func(hp *HasherPlugin) { hp.priority = p }
}

// WithHasherPluginName sets a custom plugin name.
func WithHasherPluginName(n string) HasherPluginOption {
	return func(hp *HasherPlugin) { hp.pluginName = n }
}

// WithHasherHeaderName sets the header name for the digest.
func WithHasherHeaderName(h string) HasherPluginOption {
	return func(hp *HasherPlugin) { hp.headerName = h }
}

// NewHasherPlugin wraps a Hasher as a plugin.Plugin.
func NewHasherPlugin(h Hasher, opts ...HasherPluginOption) *HasherPlugin {
	hp := &HasherPlugin{
		hasher:     h,
		priority:   2,
		pluginName: h.Algorithm() + "-signer",
		headerName: "X-Signature",
	}
	for _, opt := range opts {
		opt(hp)
	}
	return hp
}

// Name implements plugin.Plugin.
func (hp *HasherPlugin) Name() string { return hp.pluginName }

// Priority implements plugin.Plugin.
func (hp *HasherPlugin) Priority() int { return hp.priority }

// Before computes the hash of the request body and adds it as a
// header.
func (hp *HasherPlugin) Before(ctx *plugin.Context) error {
	req, ok := ctx.Request().(*http.HTTPRequest)
	if !ok {
		return nil
	}

	digest := hp.hasher.Hash(req.Body)
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers[hp.headerName] = digest
	return nil
}

// After is a no-op for the hasher.
func (hp *HasherPlugin) After(_ *plugin.Context) error { return nil }

// AlgorithmName returns the underlying algorithm name.
func (hp *HasherPlugin) AlgorithmName() string { return hp.hasher.Algorithm() }

// VerifierPlugin adapts a Verifier into a plugin.Plugin that checks
// the response signature header in the After phase.
type VerifierPlugin struct {
	verifier   Verifier
	priority   int
	pluginName string
	headerName string
}

// VerifierPluginOption configures a VerifierPlugin.
type VerifierPluginOption func(*VerifierPlugin)

// WithVerifierPriority sets the plugin priority.
func WithVerifierPriority(p int) VerifierPluginOption {
	return func(vp *VerifierPlugin) { vp.priority = p }
}

// WithVerifierPluginName sets a custom plugin name.
func WithVerifierPluginName(n string) VerifierPluginOption {
	return func(vp *VerifierPlugin) { vp.pluginName = n }
}

// WithVerifierHeaderName sets the header name for the signature.
func WithVerifierHeaderName(h string) VerifierPluginOption {
	return func(vp *VerifierPlugin) { vp.headerName = h }
}

// NewVerifierPlugin wraps a Verifier as a plugin.Plugin.
func NewVerifierPlugin(v Verifier, opts ...VerifierPluginOption) *VerifierPlugin {
	vp := &VerifierPlugin{
		verifier:   v,
		priority:   98,
		pluginName: v.Algorithm() + "-verifier",
		headerName: "X-Signature",
	}
	for _, opt := range opts {
		opt(vp)
	}
	return vp
}

// Name implements plugin.Plugin.
func (vp *VerifierPlugin) Name() string { return vp.pluginName }

// Priority implements plugin.Plugin.
func (vp *VerifierPlugin) Priority() int { return vp.priority }

// Before is a no-op for the verifier.
func (vp *VerifierPlugin) Before(_ *plugin.Context) error { return nil }

// After verifies the signature in the response header.
func (vp *VerifierPlugin) After(ctx *plugin.Context) error {
	resp, ok := ctx.Response().(*http.HTTPResponse)
	if !ok {
		return nil
	}

	sigHeaders, exists := resp.Headers[vp.headerName]
	if !exists || len(sigHeaders) == 0 {
		return errors.New("crypto: missing signature header")
	}

	if !vp.verifier.Verify(resp.Body, sigHeaders[0]) {
		return fmt.Errorf("crypto: signature mismatch (%s)", vp.verifier.Algorithm())
	}
	return nil
}

// AlgorithmName returns the underlying algorithm name.
func (vp *VerifierPlugin) AlgorithmName() string { return vp.verifier.Algorithm() }

// VerifyHex is a helper that computes the hex-encoded hash of data and
// compares it with the expected hex string using a constant-time
// comparison when possible.
func VerifyHex(h Hasher, data []byte, expectedHex string) bool {
	computed := h.Hash(data)
	if len(computed) != len(expectedHex) {
		return false
	}
	for i := range computed {
		if computed[i] != expectedHex[i] {
			return false
		}
	}
	return true
}

// compile-time checks
var (
	_ plugin.Plugin = (*HasherPlugin)(nil)
	_ plugin.Plugin = (*VerifierPlugin)(nil)
)
