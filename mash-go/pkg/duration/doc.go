// Package duration implements duration timer management for MASH commands.
//
// Duration timers allow commands like SetLimit, SetSetpoint, and Pause to
// automatically expire after a specified time. This provides predictable
// control without requiring explicit clear commands.
//
// # Timer Lifecycle
//
// Timer starts when the device receives the command (not when response is sent).
// When the timer expires, the corresponding value is automatically cleared,
// effective values are recalculated, and subscribers are notified.
//
// # Per-Zone Tracking
//
// Timers are tracked per (ZoneID, CommandType) pair. Each zone's timers are
// independent, so a Zone 1 timer expiring doesn't affect Zone 2's values.
//
// # Timer Replacement
//
// A new command with duration replaces any existing timer for the same
// (ZoneID, CommandType). There is no stacking or accumulation.
//
// # Connection Loss
//
// Duration timers are NOT persisted across connection loss. When a zone
// disconnects, all its pending timers are cancelled. This differs from
// failsafe which has its own persistence rules.
//
// # Accuracy
//
// Timer accuracy is +/- 1% or +/- 1 second, whichever is greater.
// Devices should use monotonic time to avoid clock adjustment issues.
package duration
