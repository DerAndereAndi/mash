// Package failsafe implements failsafe timer management for MASH devices.
//
// When a device loses connection to its controller(s), it must ensure safe
// operation. The failsafe timer tracks how long the device has been disconnected
// and applies failsafe limits when the timer expires.
//
// # Failsafe Duration
//
// Configurable range: 2 hours to 24 hours (default: 4 hours)
//
// The failsafe duration is set by the controller during commissioning and
// represents the maximum time a device should operate without controller
// communication before entering failsafe mode.
//
// # Timer Behavior
//
//   - Starts when connection to ALL zones is lost
//   - Resets when any zone reconnects
//   - Triggers failsafe mode on expiry
//   - Accuracy: +/- 1% or 60 seconds, whichever is greater
//
// # Failsafe Mode
//
// When the timer expires, the device:
//   - Applies configured failsafe limits (consumption and/or production)
//   - Sets ControlState to FAILSAFE
//   - Continues normal operation within those limits
//   - Exits failsafe when any zone reconnects
//
// # Persistence (Optional)
//
// Devices may optionally persist failsafe timer state across restarts.
// This is a PICS capability (FAILSAFE.PERSIST).
//
// # Grace Period (Optional)
//
// Some devices support a grace period (default: 5 minutes) after reconnection
// before the controller must send new limits. This allows the controller to
// re-establish its understanding of device state.
package failsafe
