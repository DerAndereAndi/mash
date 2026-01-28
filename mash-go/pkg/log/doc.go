// Package log provides structured protocol logging for MASH.
//
// This package defines the Logger interface and Event types for capturing
// protocol-level events at multiple layers (transport, wire, service).
// It is separate from operational logging (slog) - protocol capture provides
// a complete machine-readable event trace for debugging and analysis.
//
// # Basic Usage
//
// Applications configure logging by providing a Logger implementation:
//
//	// For development: log to console via slog
//	cfg.ProtocolLogger = log.NewSlogAdapter(slog.Default())
//
//	// For production: write to binary file
//	cfg.ProtocolLogger, _ = log.NewFileLogger("/var/log/mash/device.mlog")
//
//	// Both: use MultiLogger
//	cfg.ProtocolLogger = log.NewMultiLogger(
//	    log.NewSlogAdapter(slog.Default()),
//	    log.NewFileLogger("/var/log/mash/device.mlog"),
//	)
//
// # Event Types
//
// Events are captured at multiple layers:
//   - Transport: Raw frame bytes (FrameEvent)
//   - Wire: Decoded messages (MessageEvent)
//   - Service: State changes (StateChangeEvent)
//
// Control messages (ping/pong/close) and errors have dedicated event types.
//
// # File Format
//
// Log files use CBOR encoding with .mlog extension. The mash-log CLI tool
// provides viewing, filtering, and export capabilities.
package log
