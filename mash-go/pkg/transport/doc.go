// Package transport provides the MASH transport layer implementation.
//
// The transport layer handles:
//   - TLS 1.3 connections with mutual authentication
//   - Length-prefixed message framing
//   - Keep-alive ping/pong for connection liveness
//   - Connection state management
//
// # Protocol Stack
//
//	┌────────────────────────────────┐
//	│      CBOR Messages             │
//	├────────────────────────────────┤
//	│   Length-Prefix Framing (4B)   │
//	├────────────────────────────────┤
//	│         TLS 1.3                │
//	├────────────────────────────────┤
//	│           TCP                  │
//	├────────────────────────────────┤
//	│         IPv6 only              │
//	└────────────────────────────────┘
//
// # TLS Requirements
//
// MASH requires TLS 1.3 with no fallback to earlier versions.
// Cipher suites (in preference order):
//   - TLS_AES_128_GCM_SHA256 (mandatory)
//   - TLS_AES_256_GCM_SHA384 (recommended)
//   - TLS_CHACHA20_POLY1305_SHA256 (optional)
//
// Key exchange:
//   - ECDHE with P-256 (mandatory)
//   - ECDHE with X25519 (recommended)
//
// # Keep-Alive
//
// Connection liveness is monitored using ping/pong messages:
//   - Ping interval: 30 seconds
//   - Pong timeout: 5 seconds
//   - Max missed pongs: 3
//   - Maximum detection delay: 95 seconds
package transport
