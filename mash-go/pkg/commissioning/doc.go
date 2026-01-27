// Package commissioning implements MASH device commissioning using SPAKE2+.
//
// # Overview
//
// Commissioning is the process of securely pairing a controller (e.g., EMS)
// with a device (e.g., EVSE). MASH uses SPAKE2+ (Password-Authenticated
// Key Exchange) to establish trust without transmitting the setup code.
//
// # SPAKE2+ Protocol
//
// SPAKE2+ is an augmented PAKE protocol where:
//   - The client (controller) knows the password (setup code)
//   - The server (device) stores only a verifier derived from the password
//   - Neither the password nor verifier is transmitted during the exchange
//   - Both parties derive the same shared secret
//
// # Setup Code
//
// The setup code is an 8-digit decimal number (00000000-99999999) providing
// approximately 27 bits of entropy. It is typically delivered via:
//   - QR code on the device
//   - Printed label
//   - Device display
//
// # QR Code Format
//
//	MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
//
// Example: MASH:1:1234:12345678:0x1234:0x5678
//
// # Commissioning Flow
//
//  1. Controller discovers device via mDNS
//  2. Controller connects to device (TLS with InsecureSkipVerify)
//  3. SPAKE2+ exchange using setup code from QR code
//  4. Shared secret established and verified
//  5. Device generates new key pair and CSR
//  6. Controller signs CSR with Zone CA
//  7. Controller installs operational certificate on device
//  8. Commissioning complete - device is now a zone member
//
// # Security Properties
//
//   - Setup code is never transmitted
//   - Resistant to offline dictionary attacks (SPAKE2+ property)
//   - Forward secrecy via ephemeral key exchange
//   - Mutual authentication after certificate installation
//
// # Cryptographic Parameters
//
//   - Curve: P-256 (NIST)
//   - Hash: SHA-256
//   - KDF: HKDF-SHA256
//   - MAC: HMAC-SHA256
package commissioning
