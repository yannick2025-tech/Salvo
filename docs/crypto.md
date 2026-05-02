# Crypto Module

Salvo 的加密模块提供两套独立的抽象体系：

- **Encryptor / Decryptor**：对称加密/解密（AES-GCM、AES-CBC、AES-CTR）
- **Hasher / Verifier**：哈希签名/验证（HMAC-SHA256）

每套体系都遵循 **接口 → 实现 → Plugin 适配器** 三层架构，方便扩展新算法。

---

## 架构总览

```
crypto/
├── aes.go            AES 统一入口 + Mode 枚举 + aesMode 内部接口
├── aes_gcm.go        GCM 模式实现（AEAD，推荐）
├── aes_cbc.go        CBC 模式实现（PKCS7 填充，需配合 HMAC）
├── aes_ctr.go        CTR 模式实现（流式，需配合 HMAC）+ PKCS7 工具
├── cipher.go         Encryptor/Decryptor 接口 + Plugin 适配器
├── hasher.go         Hasher/Verifier 接口 + Plugin 适配器
├── hmac_sha256.go    HMAC-SHA256 实现
└── *_test.go         测试文件
```

---

## 1. 加密/解密（Encryptor / Decryptor）

### 1.1 接口定义

```go
type Encryptor interface {
    Encrypt(plaintext []byte) ([]byte, error)
    Algorithm() string   // e.g. "aes-256-gcm"
}

type Decryptor interface {
    Decrypt(ciphertext []byte) ([]byte, error)
    Algorithm() string   // e.g. "aes-256-gcm"
}
```

### 1.2 AES 构造器

```go
a, err := NewAES(key, opts...)
```

**密钥长度自动推断 AES 变体：**

| key 长度 | AES 变体 | Algorithm() 返回值（GCM 模式） |
|----------|----------|-------------------------------|
| 16 bytes | AES-128  | `"aes-128-gcm"`               |
| 24 bytes | AES-192  | `"aes-192-gcm"`               |
| 32 bytes | AES-256  | `"aes-256-gcm"`               |

**模式选择：**

```go
a, _ := NewAES(key32)                        // 默认 GCM
a, _ := NewAES(key32, WithMode(ModeGCM))     // 显式 GCM
a, _ := NewAES(key32, WithMode(ModeCBC))     // CBC 模式
a, _ := NewAES(key16, WithMode(ModeCTR))     // AES-128-CTR
```

### 1.3 三种模式对比

| 模式 | 类型 | 认证 | 填充 | 并行 | 推荐场景 |
|------|------|------|------|------|----------|
| `ModeGCM` | AEAD | ✅ 内置 | 无需 | ✅ | **新系统首选**，加密+认证一体 |
| `ModeCBC` | 基础 | ❌ 需 HMAC | PKCS7 | ❌ | 旧系统兼容 |
| `ModeCTR` | 基础 | ❌ 需 HMAC | 无需 | ✅ | 流式数据，随机访问 |

> **安全提示**：CBC 和 CTR 不提供认证，密文可被篡改而不被发现。生产环境请使用 GCM，或搭配 HasherPlugin 实现 encrypt-then-MAC 模式。

### 1.4 直接使用示例

```go
key := []byte("32-byte-key-12345678901234567890")  // 32 bytes → AES-256

a, err := NewAES(key, WithMode(ModeGCM))
if err != nil {
    log.Fatal(err)
}

// 加密
ciphertext, err := a.Encrypt([]byte("hello world"))

// 解密
plaintext, err := a.Decrypt(ciphertext)
```

### 1.5 Plugin 适配器

```go
// EncryptorPlugin：Before 阶段加密请求 Body
ep := NewEncryptorPlugin(a,
    WithEncryptorPriority(5),           // 优先级（默认 5）
    WithEncryptorPluginName("custom"),  // 自定义名称
)

// DecryptorPlugin：After 阶段解密响应 Body
dp := NewDecryptorPlugin(a,
    WithDecryptorPriority(95),          // 优先级（默认 95）
    WithDecryptorPluginName("custom"),  // 自定义名称
)

// 注册到 Plugin Registry
registry.Register(ep)
registry.Register(dp)
```

**Plugin 执行流程：**

```
请求 → [EncryptorPlugin.Before: 加密 Body] → 发送 → 接收 → [DecryptorPlugin.After: 解密 Body]
```

---

## 2. 哈希签名/验证（Hasher / Verifier）

### 2.1 接口定义

```go
type Hasher interface {
    Hash(data []byte) string     // 返回 hex 编码的摘要
    Algorithm() string           // e.g. "hmac-sha256"
}

type Verifier interface {
    Verify(data []byte, expectedHex string) bool
    Algorithm() string
}
```

### 2.2 HMAC-SHA256

```go
h := NewHMACSHA256(key)

// 签名
digest := h.Hash([]byte("data to sign"))  // → "a1b2c3..."

// 验证
ok := h.Verify([]byte("data to sign"), digest)  // → true
```

### 2.3 Plugin 适配器

```go
// HasherPlugin：Before 阶段签名请求 Body，写入 Header
hp := NewHasherPlugin(h,
    WithHasherPriority(2),              // 优先级（默认 2）
    WithHasherHeaderName("X-My-Sig"),   // 签名 Header 名（默认 "X-Signature"）
)

// VerifierPlugin：After 阶段验证响应签名
vp := NewVerifierPlugin(h,
    WithVerifierPriority(98),           // 优先级（默认 98）
    WithVerifierHeaderName("X-My-Sig"), // 签名 Header 名（默认 "X-Signature"）
)

registry.Register(hp)
registry.Register(vp)
```

**Plugin 执行流程：**

```
请求 → [HasherPlugin.Before: 计算签名写入 X-Signature Header] → 发送 → 接收 → [VerifierPlugin.After: 验证响应签名]
```

### 2.4 工具函数

```go
// VerifyHex：通用 hex 摘要比较
ok := VerifyHex(h, data, expectedHex)
```

---

## 3. 组合使用：Encrypt-then-MAC

CBC/CTR 模式不提供认证，推荐搭配 HasherPlugin 实现 encrypt-then-MAC：

```go
key := []byte("32-byte-key-12345678901234567890")
hmacKey := []byte("hmac-key-for-signing")

a, _ := NewAES(key, WithMode(ModeCBC))
h := NewHMACSHA256(hmacKey)

// 注册顺序（按 priority 升序执行 Before，降序执行 After）：
// Before: Hasher(2) → Encryptor(5)  → 先签名原文，再加密
// After:  Decryptor(95) → Verifier(98) → 先解密，再验证签名
registry.Register(NewHasherPlugin(h))       // priority=2
registry.Register(NewEncryptorPlugin(a))    // priority=5
registry.Register(NewDecryptorPlugin(a))    // priority=95
registry.Register(NewVerifierPlugin(h))     // priority=98
```

> **注意**：Hasher 的 priority(2) < Encryptor 的 priority(5)，确保 Before 阶段先签名原文再加密。GCM 模式自带认证，不需要此组合。

---

## 4. 扩展新算法

### 4.1 新增 AES 模式（如 CFB）

**步骤 1**：在 [aes.go](internal/plugin/crypto/aes.go) 的 `Mode` 枚举中添加：

```go
const (
    ModeGCM Mode = iota
    ModeCBC
    ModeCTR
    ModeCFB   // 新增
)
```

**步骤 2**：新建 `aes_cfb.go`，实现 `aesMode` 内部接口：

```go
package crypto

import "crypto/cipher"

type cfbMode struct{}

func (c *cfbMode) encrypt(block cipher.Block, plaintext []byte) ([]byte, error) {
    // CFB 加密实现
}

func (c *cfbMode) decrypt(block cipher.Block, ciphertext []byte) ([]byte, error) {
    // CFB 解密实现
}
```

**步骤 3**：在 `newAESMode()` 中注册：

```go
case ModeCFB:
    return &cfbMode{}, nil
```

无需修改 `Encryptor`/`Decryptor` 接口或 Plugin 适配器，`NewAES(key, WithMode(ModeCFB))` 即可使用。

### 4.2 新增加密算法（如 ChaCha20-Poly1305）

**步骤 1**：新建 `chacha20.go`，实现 `Encryptor` + `Decryptor`：

```go
package crypto

type ChaCha20Poly1305 struct { key []byte }

func NewChaCha20Poly1305(key []byte) (*ChaCha20Poly1305, error) { ... }
func (c *ChaCha20Poly1305) Algorithm() string { return "chacha20-poly1305" }
func (c *ChaCha20Poly1305) Encrypt(plaintext []byte) ([]byte, error) { ... }
func (c *ChaCha20Poly1305) Decrypt(ciphertext []byte) ([]byte, error) { ... }
```

**步骤 2**：直接使用现有 Plugin 适配器：

```go
c, _ := NewChaCha20Poly1305(key)
ep := NewEncryptorPlugin(c)  // 自动适配
dp := NewDecryptorPlugin(c)  // 自动适配
```

### 4.3 新增哈希算法（如 SHA-256 纯哈希、bcrypt、Ed25519）

**步骤 1**：新建文件，实现 `Hasher` + `Verifier`：

```go
package crypto

type BcryptHasher struct { cost int }

func NewBcryptHasher(cost int) *BcryptHasher { ... }
func (b *BcryptHasher) Algorithm() string { return "bcrypt" }
func (b *BcryptHasher) Hash(data []byte) string { ... }
func (b *BcryptHasher) Verify(data []byte, expectedHex string) bool { ... }
```

**步骤 2**：直接使用现有 Plugin 适配器：

```go
b := NewBcryptHasher(10)
hp := NewHasherPlugin(b)   // 自动适配
vp := NewVerifierPlugin(b) // 自动适配
```

---

## 5. 扩展路线图

| 类别 | 算法 | 类型 | 优先级 |
|------|------|------|--------|
| AES 模式 | CFB (密文反馈) | 基础模式 | 低 |
| AES 模式 | OFB (输出反馈) | 基础模式 | 低 |
| AEAD | ChaCha20-Poly1305 | 认证加密 | 高 |
| AEAD | AES-CCM | 认证加密 | 中 |
| 哈希 | SHA-256/SHA-512 | 纯哈希（无密钥） | 高 |
| 哈希 | HMAC-SHA512 | HMAC | 中 |
| 密码哈希 | bcrypt | 密码存储 | 高 |
| 密码哈希 | scrypt / argon2 | 密码存储 | 中 |
| 非对称签名 | Ed25519 | 签名/验证 | 中 |
| 非对称签名 | RSA-PSS | 签名/验证 | 低 |
