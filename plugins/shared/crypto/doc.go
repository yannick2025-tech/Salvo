// Package crypto provides business-specific encryption utilities shared across
// Salvo SO plugins.
//
// 此包为业务定制版加密工具，与 internal/plugin/crypto 的区别：
//
//   - internal/plugin/crypto: 通用加密包，使用随机 IV/nonce，符合标准库惯例，
//     供 Salvo 内部加密插件使用。
//   - plugins/shared/crypto (本包): 业务定制版，使用外部传入 IV/nonce，
//     base64 编码 I/O，兼容业务系统（如 login.py）的加解密格式。
//
// 业务背景：业务系统的 AES-GCM 使用 16 字节 nonce（非标准 12 字节），
// 且 IV 由调用方传入（通常取 key 的前 16 字节），与标准库默认行为不同。
// CBC 模式则遵循 login.py 约定：IV 前置到密文，整体 base64 编码。
package crypto
