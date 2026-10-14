// Package crypto provides general-purpose encryption utilities for Salvo's
// internal encryption plugin.
//
// 定位：通用加密包，遵循标准库惯例，使用随机 IV/nonce。
// 供 Salvo 内部的 crypto 插件（${__so("crypto", ...)}）使用。
//
// 与 plugins/shared/crypto（业务定制版）的区别：
//
//   - internal/plugin/crypto (本包): 通用版，随机 IV/nonce，符合标准库默认行为。
//     适合需要遵循最佳实践的新场景。
//   - plugins/shared/crypto: 业务定制版，使用外部传入 IV/nonce，base64 I/O，
//     兼容已有业务系统（如 login.py）的加解密格式。供业务 SO 插件
//     (login/paypwd/aes) 使用。
//
// 两套实现保持独立，避免语义混杂。新增加密需求时根据场景选择：
//   - 对接已有业务系统 → plugins/shared/crypto
//   - Salvo 内部新功能 → internal/plugin/crypto
package crypto
