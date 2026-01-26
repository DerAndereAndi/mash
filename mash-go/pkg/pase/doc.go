// Package pase implements the PASE (Password-Authenticated Session Establishment)
// window for MASH commissioning.
//
// # Overview
//
// PASE is the commissioning phase where a controller pairs with a device using
// a setup code. The pase package manages the commissioning window state machine
// that controls when devices accept commissioning attempts.
//
// The SPAKE2+ cryptographic protocol is implemented in the commissioning package.
// This package handles the window lifecycle and session management.
//
// # Window States
//
//   - CLOSED: Device is not accepting commissioning (normal operation)
//   - OPEN: Device is accepting commissioning attempts (window open)
//   - PASE_IN_PROGRESS: A SPAKE2+ exchange is in progress
//
// # Window Lifecycle
//
//  1. User triggers window open (physical button, command, factory reset)
//  2. Window opens for a configurable timeout (default 120 seconds)
//  3. Controller initiates SPAKE2+ exchange
//  4. State moves to PASE_IN_PROGRESS (only one concurrent session)
//  5. On success: window closes, device is commissioned
//  6. On failure: returns to OPEN (if timeout not expired)
//  7. On timeout: window closes
//
// # Security Properties
//
//   - Only one concurrent PASE session allowed
//   - Window auto-closes after timeout
//   - Failed attempts don't extend the window
package pase
