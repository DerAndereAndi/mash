// Package connection provides connection lifecycle management for MASH.
//
// This package handles:
//   - Exponential backoff for reconnection attempts
//   - Jitter to prevent thundering herd
//   - Connection state tracking
//   - Automatic reconnection on connection loss
//
// # Reconnection Strategy
//
// When a connection is lost, the client uses exponential backoff:
//
//  1. Initial delay: 1 second
//  2. Exponential increase: 2s, 4s, 8s, 16s, 32s
//  3. Maximum delay: 60 seconds
//  4. Continue at 60s until successful
//  5. Reset to 1s on successful reconnection
//
// # Jitter
//
// To prevent thundering herd when multiple clients reconnect:
//
//	actual_delay = base_delay + random(0, base_delay * 0.25)
//
// # Success Criteria
//
// A reconnection is successful when:
//   - TCP connection established
//   - TLS 1.3 handshake completed
//   - Both certificates validated
//   - Device not in commissioning mode
//
// Application-layer rejection after TLS success does NOT reset backoff.
package connection
