// Package crypto provides encryption and decryption plugins for Salvo.
// It supports AES-GCM for request body encryption and HMAC signing
// for request integrity verification.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/yannick2025-tech/Salvo/internal/plugin"
	"github.com/yannick2025-tech/Salvo/internal/protocol/http"
)

// AESGCMEncryptor encrypts the request body with AES-256-GCM before
// the request is sent.
type AESGCMEncryptor struct {
	name     string
	priority int
	key      []byte
}

// AESGCMOption configures an AESGCMEncryptor.
type AESGCMOption func(*AESGCMEncryptor)

// WithAESGCMName sets a custom plugin name.
func WithAESGCMName(n string) AESGCMOption {
	return func(e *AESGCMEncryptor) { e.name = n }
}

// WithAESGCMPriority sets the plugin priority.
func WithAESGCMPriority(p int) AESGCMOption {
	return func(e *AESGCMEncryptor) { e.priority = p }
}

// NewAESGCMEncryptor creates an AES-GCM encryptor plugin. The key must
// be exactly 32 bytes (AES-256).
func NewAESGCMEncryptor(key []byte, opts ...AESGCMOption) (*AESGCMEncryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: AES-256 key must be 32 bytes")
	}
	e := &AESGCMEncryptor{
		name:     "aes-gcm-encryptor",
		priority: 5,
		key:      make([]byte, 32),
	}
	copy(e.key, key)
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Name implements plugin.Plugin.
func (e *AESGCMEncryptor) Name() string { return e.name }

// Priority implements plugin.Plugin.
func (e *AESGCMEncryptor) Priority() int { return e.priority }

// Before encrypts the request body if it is an HTTP request.
func (e *AESGCMEncryptor) Before(ctx *plugin.Context) error {
	req, ok := ctx.Request().(*http.HTTPRequest)
	if !ok {
		return nil
	}
	if len(req.Body) == 0 {
		return nil
	}

	encrypted, err := encryptAESGCM(e.key, req.Body)
	if err != nil {
		return fmt.Errorf("crypto: encrypt: %w", err)
	}
	req.Body = encrypted
	return nil
}

// After is a no-op for the encryptor.
func (e *AESGCMEncryptor) After(_ *plugin.Context) error {
	return nil
}

// AESGCMDecryptor decrypts the response body with AES-256-GCM after
// the response is received.
type AESGCMDecryptor struct {
	name     string
	priority int
	key      []byte
}

// AESGCMDecryptorOption configures an AESGCMDecryptor.
type AESGCMDecryptorOption func(*AESGCMDecryptor)

// WithAESGCMDecryptorName sets a custom plugin name.
func WithAESGCMDecryptorName(n string) AESGCMDecryptorOption {
	return func(d *AESGCMDecryptor) { d.name = n }
}

// WithAESGCMDecryptorPriority sets the plugin priority.
func WithAESGCMDecryptorPriority(p int) AESGCMDecryptorOption {
	return func(d *AESGCMDecryptor) { d.priority = p }
}

// NewAESGCMDecryptor creates an AES-GCM decryptor plugin.
func NewAESGCMDecryptor(key []byte, opts ...AESGCMDecryptorOption) (*AESGCMDecryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: AES-256 key must be 32 bytes")
	}
	d := &AESGCMDecryptor{
		name:     "aes-gcm-decryptor",
		priority: 95,
		key:      make([]byte, 32),
	}
	copy(d.key, key)
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

// Name implements plugin.Plugin.
func (d *AESGCMDecryptor) Name() string { return d.name }

// Priority implements plugin.Plugin.
func (d *AESGCMDecryptor) Priority() int { return d.priority }

// Before is a no-op for the decryptor.
func (d *AESGCMDecryptor) Before(_ *plugin.Context) error {
	return nil
}

// After decrypts the response body if it is an HTTP response.
func (d *AESGCMDecryptor) After(ctx *plugin.Context) error {
	resp, ok := ctx.Response().(*http.HTTPResponse)
	if !ok {
		return nil
	}
	if len(resp.Body) == 0 {
		return nil
	}

	decrypted, err := decryptAESGCM(d.key, resp.Body)
	if err != nil {
		return fmt.Errorf("crypto: decrypt: %w", err)
	}
	resp.Body = decrypted
	return nil
}

// HMACSigner signs the request body with HMAC-SHA256 and adds the
// signature as a header.
type HMACSigner struct {
	name       string
	priority   int
	key        []byte
	headerName string
}

// HMACSignerOption configures an HMACSigner.
type HMACSignerOption func(*HMACSigner)

// WithHMACSignerName sets a custom plugin name.
func WithHMACSignerName(n string) HMACSignerOption {
	return func(s *HMACSigner) { s.name = n }
}

// WithHMACSignerPriority sets the plugin priority.
func WithHMACSignerPriority(p int) HMACSignerOption {
	return func(s *HMACSigner) { s.priority = p }
}

// WithHMACHeaderName sets the header name for the signature.
func WithHMACHeaderName(h string) HMACSignerOption {
	return func(s *HMACSigner) { s.headerName = h }
}

// NewHMACSigner creates an HMAC-SHA256 signing plugin.
func NewHMACSigner(key []byte, opts ...HMACSignerOption) *HMACSigner {
	s := &HMACSigner{
		name:       "hmac-signer",
		priority:   2,
		key:        make([]byte, len(key)),
		headerName: "X-Signature",
	}
	copy(s.key, key)
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Name implements plugin.Plugin.
func (s *HMACSigner) Name() string { return s.name }

// Priority implements plugin.Plugin.
func (s *HMACSigner) Priority() int { return s.priority }

// Before computes the HMAC-SHA256 of the request body and adds it as
// a header.
func (s *HMACSigner) Before(ctx *plugin.Context) error {
	req, ok := ctx.Request().(*http.HTTPRequest)
	if !ok {
		return nil
	}

	mac := hmac.New(sha256.New, s.key)
	mac.Write(req.Body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers[s.headerName] = sig
	return nil
}

// After is a no-op for the signer.
func (s *HMACSigner) After(_ *plugin.Context) error {
	return nil
}

// HMACVerifier verifies the HMAC-SHA256 signature in the response
// header.
type HMACVerifier struct {
	name       string
	priority   int
	key        []byte
	headerName string
}

// HMACVerifierOption configures an HMACVerifier.
type HMACVerifierOption func(*HMACVerifier)

// WithHMACVerifierName sets a custom plugin name.
func WithHMACVerifierName(n string) HMACVerifierOption {
	return func(v *HMACVerifier) { v.name = n }
}

// WithHMACVerifierPriority sets the plugin priority.
func WithHMACVerifierPriority(p int) HMACVerifierOption {
	return func(v *HMACVerifier) { v.priority = p }
}

// WithHMACVerifierHeaderName sets the header name for the signature.
func WithHMACVerifierHeaderName(h string) HMACVerifierOption {
	return func(v *HMACVerifier) { v.headerName = h }
}

// NewHMACVerifier creates an HMAC-SHA256 verification plugin.
func NewHMACVerifier(key []byte, opts ...HMACVerifierOption) *HMACVerifier {
	v := &HMACVerifier{
		name:       "hmac-verifier",
		priority:   98,
		key:        make([]byte, len(key)),
		headerName: "X-Signature",
	}
	copy(v.key, key)
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Name implements plugin.Plugin.
func (v *HMACVerifier) Name() string { return v.name }

// Priority implements plugin.Plugin.
func (v *HMACVerifier) Priority() int { return v.priority }

// Before is a no-op for the verifier.
func (v *HMACVerifier) Before(_ *plugin.Context) error {
	return nil
}

// After verifies the HMAC-SHA256 signature in the response header.
func (v *HMACVerifier) After(ctx *plugin.Context) error {
	resp, ok := ctx.Response().(*http.HTTPResponse)
	if !ok {
		return nil
	}

	sigHeaders, exists := resp.Headers[v.headerName]
	if !exists || len(sigHeaders) == 0 {
		return errors.New("crypto: missing signature header")
	}

	mac := hmac.New(sha256.New, v.key)
	mac.Write(resp.Body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigHeaders[0]), []byte(expected)) {
		return errors.New("crypto: signature mismatch")
	}
	return nil
}

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
