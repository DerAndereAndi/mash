package log

import (
	"context"
	"log/slog"
)

// SlogAdapter writes protocol events to an slog.Logger.
// Useful for development when you want to see protocol events in console.
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new SlogAdapter that writes to the given slog.Logger.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

// Log writes the event to the slog logger at Debug level.
func (a *SlogAdapter) Log(event Event) {
	attrs := []slog.Attr{
		slog.String("conn_id", event.ConnectionID),
		slog.String("direction", event.Direction.String()),
		slog.String("layer", event.Layer.String()),
		slog.String("category", event.Category.String()),
	}

	// Add optional identifiers
	if event.DeviceID != "" {
		attrs = append(attrs, slog.String("device_id", event.DeviceID))
	}
	if event.ZoneID != "" {
		attrs = append(attrs, slog.String("zone_id", event.ZoneID))
	}

	// Add type-specific attributes
	switch {
	case event.Frame != nil:
		attrs = append(attrs,
			slog.Int("frame_size", event.Frame.Size),
			slog.Bool("truncated", event.Frame.Truncated),
		)
	case event.Message != nil:
		attrs = append(attrs,
			slog.Uint64("msg_id", uint64(event.Message.MessageID)),
			slog.String("msg_type", event.Message.Type.String()),
		)
		if event.Message.Operation != nil {
			attrs = append(attrs, slog.String("operation", event.Message.Operation.String()))
		}
		if event.Message.EndpointID != nil {
			attrs = append(attrs, slog.Uint64("endpoint", uint64(*event.Message.EndpointID)))
		}
		if event.Message.FeatureID != nil {
			attrs = append(attrs, slog.Uint64("feature", uint64(*event.Message.FeatureID)))
		}
		if event.Message.Status != nil {
			attrs = append(attrs, slog.String("status", event.Message.Status.String()))
		}
		if event.Message.ProcessingTime != nil {
			attrs = append(attrs, slog.Duration("processing_time", *event.Message.ProcessingTime))
		}
	case event.StateChange != nil:
		attrs = append(attrs,
			slog.String("entity", event.StateChange.Entity.String()),
			slog.String("old_state", event.StateChange.OldState),
			slog.String("new_state", event.StateChange.NewState),
		)
		if event.StateChange.Reason != "" {
			attrs = append(attrs, slog.String("reason", event.StateChange.Reason))
		}
	case event.ControlMsg != nil:
		attrs = append(attrs, slog.String("ctrl_type", event.ControlMsg.Type.String()))
	case event.Error != nil:
		attrs = append(attrs,
			slog.String("error_layer", event.Error.Layer.String()),
			slog.String("error_msg", event.Error.Message),
			slog.String("error_context", event.Error.Context),
		)
		if event.Error.Code != nil {
			attrs = append(attrs, slog.Int("error_code", *event.Error.Code))
		}
	}

	a.logger.LogAttrs(context.Background(), slog.LevelDebug, "protocol", attrs...)
}

// Compile-time interface satisfaction check.
var _ Logger = (*SlogAdapter)(nil)
