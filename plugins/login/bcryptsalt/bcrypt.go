// Package bcryptsalt is a modified version of golang.org/x/crypto/bcrypt
// that supports passing a custom salt parameter.
//
// Modified function signature:
//
//	GenerateFromPassword(password []byte, salt string, cost int) ([]byte, error)
//
// The salt parameter should be in format "$2a$10$saltstring..." (the full bcrypt salt string).
package bcryptsalt

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/blowfish"
)

const (
	MinCost     int = 4
	MaxCost     int = 31
	DefaultCost int = 10
)

const (
	majorVersion       = '2'
	minorVersion       = 'a'
	maxSaltSize        = 16
	maxCryptedHashSize = 23
	encodedSaltSize    = 22
	encodedHashSize    = 31
	minHashSize        = 59
)

var magicCipherData = []byte{
	0x4f, 0x72, 0x70, 0x68,
	0x65, 0x61, 0x6e, 0x42,
	0x65, 0x68, 0x6f, 0x6c,
	0x64, 0x65, 0x72, 0x53,
	0x63, 0x72, 0x79, 0x44,
	0x6f, 0x75, 0x62, 0x74,
}

type hashed struct {
	hash  []byte
	salt  []byte
	cost  int
	major byte
	minor byte
}

// bcryptBase64 uses a non-standard alphabet (./ABC... instead of ABC...+/)
const bcryptBase64Alphabet = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

var bcryptBase64 = base64.NewEncoding(bcryptBase64Alphabet).WithPadding(base64.NoPadding)

func base64Encode(src []byte) []byte {
	return []byte(bcryptBase64.EncodeToString(src))
}

func base64Decode(src []byte) ([]byte, error) {
	return bcryptBase64.DecodeString(string(src))
}

// GenerateFromPassword generates bcrypt hash using the provided salt string.
// salt can be in format "$2a$10$saltstring..." (full bcrypt salt including prefix)
// or just the base64-encoded salt portion (e.g. "XXXXXXXXXXXXXXXXXXXXXX").
// If salt contains "$", the last segment after "$" is used as the raw salt.
func GenerateFromPassword(password []byte, salt string, cost int) ([]byte, error) {
	if len(password) > 72 {
		return nil, errors.New("bcrypt: password length exceeds 72 bytes")
	}

	// Extract pure base64 salt portion if full salt string is provided.
	rawSalt := salt
	if strings.Contains(salt, "$") {
		parts := strings.Split(salt, "$")
		rawSalt = parts[len(parts)-1]
	}

	p := new(hashed)
	p.major = majorVersion
	p.minor = minorVersion

	if cost < MinCost {
		cost = DefaultCost
	}
	p.cost = cost
	p.salt = []byte(rawSalt)

	hash, err := bcryptInternal(password, p.cost, p.salt)
	if err != nil {
		return nil, err
	}
	p.hash = hash
	return p.Hash(), nil
}

func bcryptInternal(password []byte, cost int, salt []byte) ([]byte, error) {
	cipherData := make([]byte, len(magicCipherData))
	copy(cipherData, magicCipherData)

	c, err := expensiveBlowfishSetup(password, uint32(cost), salt)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 24; i += 8 {
		for j := 0; j < 64; j++ {
			c.Encrypt(cipherData[i:i+8], cipherData[i:i+8])
		}
	}

	return base64Encode(cipherData[:maxCryptedHashSize]), nil
}

func expensiveBlowfishSetup(key []byte, cost uint32, salt []byte) (*blowfish.Cipher, error) {
	csalt, err := base64Decode(salt)
	if err != nil {
		return nil, err
	}

	ckey := append(key[:len(key):len(key)], 0)

	c, err := blowfish.NewSaltedCipher(ckey, csalt)
	if err != nil {
		return nil, err
	}

	var i, rounds uint64
	rounds = 1 << cost
	for i = 0; i < rounds; i++ {
		blowfish.ExpandKey(ckey, c)
		blowfish.ExpandKey(csalt, c)
	}

	return c, nil
}

func (p *hashed) Hash() []byte {
	arr := make([]byte, 60)
	arr[0] = '$'
	arr[1] = p.major
	n := 2
	if p.minor != 0 {
		arr[2] = p.minor
		n = 3
	}
	arr[n] = '$'
	n++
	copy(arr[n:], []byte(fmt.Sprintf("%02d", p.cost)))
	n += 2
	arr[n] = '$'
	n++
	copy(arr[n:], p.salt)
	n += encodedSaltSize
	copy(arr[n:], p.hash)
	n += encodedHashSize
	return arr[:n]
}

// CompareHashAndPassword compares a bcrypt hashed password with its possible
// plaintext equivalent. Returns nil on success, or an error on failure.
func CompareHashAndPassword(hashedPassword, password []byte) error {
	p, err := newFromHash(hashedPassword)
	if err != nil {
		return err
	}

	otherHash, err := bcryptInternal(password, p.cost, p.salt)
	if err != nil {
		return err
	}

	otherP := &hashed{otherHash, p.salt, p.cost, p.major, p.minor}
	if subtle.ConstantTimeCompare(p.Hash(), otherP.Hash()) == 1 {
		return nil
	}

	return errors.New("crypto/bcrypt: hashedPassword is not the hash of the given password")
}

func newFromHash(hashedSecret []byte) (*hashed, error) {
	if len(hashedSecret) < minHashSize {
		return nil, errors.New("crypto/bcrypt: hashedSecret too short to be a bcrypted password")
	}
	p := new(hashed)
	n, err := p.decodeVersion(hashedSecret)
	if err != nil {
		return nil, err
	}
	hashedSecret = hashedSecret[n:]
	n, err = p.decodeCost(hashedSecret)
	if err != nil {
		return nil, err
	}
	hashedSecret = hashedSecret[n:]

	p.salt = make([]byte, encodedSaltSize, encodedSaltSize+2)
	copy(p.salt, hashedSecret[:encodedSaltSize])

	hashedSecret = hashedSecret[encodedSaltSize:]
	p.hash = make([]byte, len(hashedSecret))
	copy(p.hash, hashedSecret)

	return p, nil
}

func (p *hashed) decodeVersion(sbytes []byte) (int, error) {
	if sbytes[0] != '$' {
		return -1, fmt.Errorf("crypto/bcrypt: bcrypt hashes must start with '$', but hashedSecret started with '%c'", sbytes[0])
	}
	if sbytes[1] > majorVersion {
		return -1, fmt.Errorf("crypto/bcrypt: bcrypt algorithm version '%c' requested is newer than current version '%c'", sbytes[1], majorVersion)
	}
	p.major = sbytes[1]
	n := 3
	if sbytes[2] != '$' {
		p.minor = sbytes[2]
		n++
	}
	return n, nil
}

func (p *hashed) decodeCost(sbytes []byte) (int, error) {
	cost := int((sbytes[0]-'0')*10 + (sbytes[1] - '0'))
	if cost < MinCost || cost > MaxCost {
		return -1, fmt.Errorf("crypto/bcrypt: cost %d is outside allowed range (%d,%d)", cost, MinCost, MaxCost)
	}
	p.cost = cost
	return 3, nil
}
