// Package crypto — bcrypt with custom salt support.
//
// 业务背景：标准库 golang.org/x/crypto/bcrypt 使用随机 salt，
// 无法满足业务系统（如登录、支付密码）"传入固定 salt 进行 hashpw" 的需求。
// 此实现从 golang.org/x/crypto/bcrypt 修改而来，支持传入完整 salt 字符串。
package crypto

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/blowfish"
)

const (
	bcryptMinCost     int = 4
	bcryptMaxCost     int = 31
	bcryptDefaultCost int = 10
)

const (
	bcryptMajorVersion       = '2'
	bcryptMinorVersion       = 'a'
	bcryptMaxSaltSize        = 16
	bcryptMaxCryptedHashSize = 23
	bcryptEncodedSaltSize    = 22
	bcryptEncodedHashSize    = 31
	bcryptMinHashSize        = 59
)

var bcryptMagicCipherData = []byte{
	0x4f, 0x72, 0x70, 0x68,
	0x65, 0x61, 0x6e, 0x42,
	0x65, 0x68, 0x6f, 0x6c,
	0x64, 0x65, 0x72, 0x53,
	0x63, 0x72, 0x79, 0x44,
	0x6f, 0x75, 0x62, 0x74,
}

type bcryptHashed struct {
	hash  []byte
	salt  []byte
	cost  int
	major byte
	minor byte
}

// bcryptBase64 uses a non-standard alphabet (./ABC... instead of ABC...+/)
const bcryptBase64Alphabet = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

var bcryptBase64 = base64.NewEncoding(bcryptBase64Alphabet).WithPadding(base64.NoPadding)

func bcryptBase64Encode(src []byte) []byte {
	return []byte(bcryptBase64.EncodeToString(src))
}

func bcryptBase64Decode(src []byte) ([]byte, error) {
	return bcryptBase64.DecodeString(string(src))
}

// Bcrypt constants exported for convenience.
const (
	BcryptMinCost     = bcryptMinCost
	BcryptMaxCost     = bcryptMaxCost
	BcryptDefaultCost = bcryptDefaultCost
)

// GenerateFromPassword generates bcrypt hash using the provided salt string.
// salt can be in format "$2a$10$saltstring..." (full bcrypt salt including prefix)
// or just the base64-encoded salt portion (e.g. "XXXXXXXXXXXXXXXXXXXXXX").
// If salt contains "$", the last segment after "$" is used as the raw salt.
//
// 与标准库 bcrypt.GenerateFromPassword(password, cost) 的区别：
// 标准库生成随机 salt，此函数使用传入的 salt（业务系统要求固定 salt）。
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

	p := new(bcryptHashed)
	p.major = bcryptMajorVersion
	p.minor = bcryptMinorVersion

	if cost < bcryptMinCost {
		cost = bcryptDefaultCost
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
	cipherData := make([]byte, len(bcryptMagicCipherData))
	copy(cipherData, bcryptMagicCipherData)

	c, err := bcryptExpensiveBlowfishSetup(password, uint32(cost), salt)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 24; i += 8 {
		for j := 0; j < 64; j++ {
			c.Encrypt(cipherData[i:i+8], cipherData[i:i+8])
		}
	}

	return bcryptBase64Encode(cipherData[:bcryptMaxCryptedHashSize]), nil
}

func bcryptExpensiveBlowfishSetup(key []byte, cost uint32, salt []byte) (*blowfish.Cipher, error) {
	csalt, err := bcryptBase64Decode(salt)
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

func (p *bcryptHashed) Hash() []byte {
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
	n += bcryptEncodedSaltSize
	copy(arr[n:], p.hash)
	n += bcryptEncodedHashSize
	return arr[:n]
}

// BcryptCompareHashAndPassword compares a bcrypt hashed password with its
// possible plaintext equivalent. Returns nil on success, or an error on failure.
func BcryptCompareHashAndPassword(hashedPassword, password []byte) error {
	p, err := bcryptNewFromHash(hashedPassword)
	if err != nil {
		return err
	}

	otherHash, err := bcryptInternal(password, p.cost, p.salt)
	if err != nil {
		return err
	}

	otherP := &bcryptHashed{otherHash, p.salt, p.cost, p.major, p.minor}
	if subtle.ConstantTimeCompare(p.Hash(), otherP.Hash()) == 1 {
		return nil
	}

	return errors.New("crypto/bcrypt: hashedPassword is not the hash of the given password")
}

func bcryptNewFromHash(hashedSecret []byte) (*bcryptHashed, error) {
	if len(hashedSecret) < bcryptMinHashSize {
		return nil, errors.New("crypto/bcrypt: hashedSecret too short to be a bcrypted password")
	}
	p := new(bcryptHashed)
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

	p.salt = make([]byte, bcryptEncodedSaltSize, bcryptEncodedSaltSize+2)
	copy(p.salt, hashedSecret[:bcryptEncodedSaltSize])

	hashedSecret = hashedSecret[bcryptEncodedSaltSize:]
	p.hash = make([]byte, len(hashedSecret))
	copy(p.hash, hashedSecret)

	return p, nil
}

func (p *bcryptHashed) decodeVersion(sbytes []byte) (int, error) {
	if sbytes[0] != '$' {
		return -1, fmt.Errorf("crypto/bcrypt: bcrypt hashes must start with '$', but hashedSecret started with '%c'", sbytes[0])
	}
	if sbytes[1] > bcryptMajorVersion {
		return -1, fmt.Errorf("crypto/bcrypt: bcrypt algorithm version '%c' requested is newer than current version '%c'", sbytes[1], bcryptMajorVersion)
	}
	p.major = sbytes[1]
	n := 3
	if sbytes[2] != '$' {
		p.minor = sbytes[2]
		n++
	}
	return n, nil
}

func (p *bcryptHashed) decodeCost(sbytes []byte) (int, error) {
	cost := int((sbytes[0]-'0')*10 + (sbytes[1] - '0'))
	if cost < bcryptMinCost || cost > bcryptMaxCost {
		return -1, fmt.Errorf("crypto/bcrypt: cost %d is outside allowed range (%d,%d)", cost, bcryptMinCost, bcryptMaxCost)
	}
	p.cost = cost
	return 3, nil
}
