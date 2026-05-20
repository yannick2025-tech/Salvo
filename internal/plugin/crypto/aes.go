package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

// Mode represents a block cipher mode of operation.
type Mode int

const (
	// ModeGCM is the Galois/Counter Mode (AEAD). It provides both
	// encryption and authentication. This is the recommended mode for
	// most use cases.
	ModeGCM Mode = iota
	// ModeCBC is the Cipher Block Chaining mode. It requires PKCS7
	// padding and does NOT provide authentication. Combine with a
	// HasherPlugin for integrity when using this mode.
	ModeCBC
	// ModeCTR is the Counter mode. It does NOT require padding but
	// does NOT provide authentication. Combine with a HasherPlugin
	// for integrity when using this mode.
	ModeCTR
)

func (m Mode) String() string {
	switch m {
	case ModeGCM:
		return "gcm"
	case ModeCBC:
		return "cbc"
	case ModeCTR:
		return "ctr"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// aesMode is the internal interface for mode-specific encrypt/decrypt.
type aesMode interface {
	encrypt(block cipher.Block, plaintext []byte) ([]byte, error)
	decrypt(block cipher.Block, ciphertext []byte) ([]byte, error)
}

// AES implements both Encryptor and Decryptor using the AES block
// cipher with a configurable mode of operation (GCM, CBC, CTR).
//
// Key length determines the AES variant:
//   - 16 bytes → AES-128
//   - 24 bytes → AES-192
//   - 32 bytes → AES-256
//
// The mode is selected via WithMode option (default: GCM).
type AES struct {
	key     []byte
	keyBits int
	mode    Mode
	impl    aesMode
}

// AESOption configures an AES cipher.
type AESOption func(*AES)

// WithMode sets the block cipher mode. Default is ModeGCM.
func WithMode(m Mode) AESOption {
	return func(a *AES) { a.mode = m }
}

// NewAES creates an AES cipher with the given key and options.
// The key length must be 16, 24, or 32 bytes for AES-128/192/256.
func NewAES(key []byte, opts ...AESOption) (*AES, error) {
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, fmt.Errorf("crypto: AES key must be 16, 24, or 32 bytes, got %d", len(key))
	}

	a := &AES{
		key:     make([]byte, len(key)),
		keyBits: len(key) * 8,
		mode:    ModeGCM,
	}
	copy(a.key, key)

	for _, opt := range opts {
		opt(a)
	}

	impl, err := newAESMode(a.mode)
	if err != nil {
		return nil, err
	}
	a.impl = impl

	return a, nil
}

// Algorithm implements Encryptor / Decryptor.
func (a *AES) Algorithm() string {
	return fmt.Sprintf("aes-%d-%s", a.keyBits, a.mode)
}

// Encrypt implements Encryptor.
func (a *AES) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	return a.impl.encrypt(block, plaintext)
}

// Decrypt implements Decryptor.
func (a *AES) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	return a.impl.decrypt(block, ciphertext)
}

// KeyBits returns the key size in bits (128, 192, or 256).
func (a *AES) KeyBits() int { return a.keyBits }

// ModeName returns the mode string (e.g. "gcm", "cbc", "ctr").
func (a *AES) ModeName() string { return a.mode.String() }

func newAESMode(m Mode) (aesMode, error) {
	switch m {
	case ModeGCM:
		return &gcmMode{}, nil
	case ModeCBC:
		return &cbcMode{}, nil
	case ModeCTR:
		return &ctrMode{}, nil
	default:
		return nil, fmt.Errorf("crypto: unsupported AES mode: %s", m)
	}
}

// compile-time checks
var (
	_ Encryptor = (*AES)(nil)
	_ Decryptor = (*AES)(nil)
)

// errCiphertextTooShort is returned when the ciphertext is shorter
// than the expected nonce/IV size.
var errCiphertextTooShort = errors.New("ciphertext too short")
