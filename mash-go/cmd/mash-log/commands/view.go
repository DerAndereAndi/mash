// Package commands implements the mash-log CLI commands.
package commands

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

// ViewFilter specifies criteria for filtering events in the view command.
type ViewFilter struct {
	Layer     *log.Layer
	Direction *log.Direction
	Category  *log.Category
}

// formatEvent writes a human-readable representation of the event to w.
func formatEvent(w io.Writer, event log.Event) {
	// Header line: timestamp [conn:id] DIRECTION LAYER Type
	ts := event.Timestamp.UTC().Format("2006-01-02T15:04:05.000000Z")
	connID := shortenConnID(event.ConnectionID)
	dir := event.Direction.String()

	// Determine event type label
	var typeLabel string
	switch {
	case event.Frame != nil:
		typeLabel = "Frame"
	case event.Message != nil:
		typeLabel = event.Message.Type.String()
	case event.StateChange != nil:
		typeLabel = "State"
	case event.ControlMsg != nil:
		typeLabel = event.ControlMsg.Type.String()
	case event.Snapshot != nil:
		typeLabel = "Snapshot"
	case event.Error != nil:
		typeLabel = "Error"
	default:
		typeLabel = "Unknown"
	}

	// Use CTRL for control messages in header
	layerStr := event.Layer.String()
	if event.Category == log.CategoryControl {
		layerStr = "CTRL"
	}

	fmt.Fprintf(w, "%s [conn:%s] %-3s %s %s\n", ts, connID, dir, layerStr, typeLabel)

	// Type-specific details
	switch {
	case event.Frame != nil:
		formatFrameDetails(w, event.Frame)
	case event.Message != nil:
		formatMessageDetails(w, event.Message)
	case event.StateChange != nil:
		formatStateChangeDetails(w, event.StateChange)
	case event.ControlMsg != nil:
		// Control messages are simple, no extra details needed
	case event.Snapshot != nil:
		formatSnapshotDetails(w, event.Snapshot)
	case event.Error != nil:
		formatErrorDetails(w, event.Error)
	}

	fmt.Fprintln(w) // Blank line between events
}

// shortenConnID returns the first 8 characters of the connection ID.
func shortenConnID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// formatFrameDetails writes frame-specific details.
func formatFrameDetails(w io.Writer, frame *log.FrameEvent) {
	fmt.Fprintf(w, "  Size: %d bytes\n", frame.Size)
	if len(frame.Data) > 0 {
		fmt.Fprintf(w, "  Data: %s", hex.EncodeToString(frame.Data))
		if frame.Truncated {
			fmt.Fprintf(w, " (truncated)")
		}
		fmt.Fprintln(w)
	}
}

// formatMessageDetails writes message-specific details.
func formatMessageDetails(w io.Writer, msg *log.MessageEvent) {
	fmt.Fprintf(w, "  MessageID: %d\n", msg.MessageID)

	switch msg.Type {
	case log.MessageTypeRequest:
		if msg.Operation != nil {
			fmt.Fprintf(w, "  Operation: %s\n", msg.Operation.String())
		}
		if msg.EndpointID != nil {
			fmt.Fprintf(w, "  Endpoint: %d", *msg.EndpointID)
			if msg.FeatureID != nil {
				fmt.Fprintf(w, "  Feature: %d", *msg.FeatureID)
			}
			fmt.Fprintln(w)
		}

	case log.MessageTypeResponse:
		if msg.Status != nil {
			fmt.Fprintf(w, "  Status: %s (%d)\n", msg.Status.String(), *msg.Status)
		}
		if msg.ProcessingTime != nil {
			fmt.Fprintf(w, "  Duration: %s\n", formatDuration(*msg.ProcessingTime))
		}

	case log.MessageTypeNotification:
		if msg.SubscriptionID != nil {
			fmt.Fprintf(w, "  SubscriptionID: %d\n", *msg.SubscriptionID)
		}
	}

	if msg.Payload != nil {
		payloadJSON, err := json.Marshal(msg.Payload)
		if err == nil {
			fmt.Fprintf(w, "  Payload: %s\n", string(payloadJSON))
		}
	}
}

// formatStateChangeDetails writes state change details.
func formatStateChangeDetails(w io.Writer, sc *log.StateChangeEvent) {
	fmt.Fprintf(w, "  Entity: %s\n", sc.Entity.String())
	if sc.OldState != "" {
		fmt.Fprintf(w, "  %s -> %s\n", sc.OldState, sc.NewState)
	} else {
		fmt.Fprintf(w, "  -> %s\n", sc.NewState)
	}
	if sc.Reason != "" {
		fmt.Fprintf(w, "  Reason: %s\n", sc.Reason)
	}
}

// formatErrorDetails writes error details.
func formatErrorDetails(w io.Writer, err *log.ErrorEventData) {
	fmt.Fprintf(w, "  Layer: %s\n", err.Layer.String())
	fmt.Fprintf(w, "  Message: %s\n", err.Message)
	if err.Code != nil {
		fmt.Fprintf(w, "  Code: %d\n", *err.Code)
	}
	if err.Context != "" {
		fmt.Fprintf(w, "  Context: %s\n", err.Context)
	}
}

// formatSnapshotDetails writes capability snapshot details.
func formatSnapshotDetails(w io.Writer, snap *log.CapabilitySnapshotEvent) {
	if snap.Local != nil {
		fmt.Fprintf(w, "  Local: %s\n", snap.Local.DeviceID)
		formatDeviceSnapshot(w, snap.Local, "    ")
	}
	if snap.Remote != nil {
		fmt.Fprintf(w, "  Remote: %s\n", snap.Remote.DeviceID)
		formatDeviceSnapshot(w, snap.Remote, "    ")
	}
}

// formatDeviceSnapshot writes the endpoint/feature tree for a device snapshot.
func formatDeviceSnapshot(w io.Writer, ds *log.DeviceSnapshot, indent string) {
	for _, ep := range ds.Endpoints {
		epTypeName := model.EndpointType(ep.Type).String()
		if ep.Label != "" {
			fmt.Fprintf(w, "%sEndpoint %d (%s) %q\n", indent, ep.ID, epTypeName, ep.Label)
		} else {
			fmt.Fprintf(w, "%sEndpoint %d (%s)\n", indent, ep.ID, epTypeName)
		}
		for _, f := range ep.Features {
			fName := model.FeatureType(f.ID).String()
			fmt.Fprintf(w, "%s  %s [featureMap=0x%04x, attrs=%d, cmds=%d]\n",
				indent, fName, f.FeatureMap, len(f.AttributeList), len(f.CommandList))
		}
	}
	if len(ds.UseCases) > 0 {
		fmt.Fprintf(w, "%sUseCases:", indent)
		for i, uc := range ds.UseCases {
			name := string(usecase.IDToName[usecase.UseCaseID(uc.ID)])
			if name == "" {
				name = fmt.Sprintf("0x%02X", uc.ID)
			}
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, " %s(%d.%d) scenarios=0x%02x", name, uc.Major, uc.Minor, uc.Scenarios)
		}
		fmt.Fprintln(w)
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.3fus", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.3fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}

// filterEvents returns events matching the filter criteria.
func filterEvents(events []log.Event, filter ViewFilter) []log.Event {
	var result []log.Event
	for _, e := range events {
		if filter.Layer != nil && e.Layer != *filter.Layer {
			continue
		}
		if filter.Direction != nil && e.Direction != *filter.Direction {
			continue
		}
		if filter.Category != nil && e.Category != *filter.Category {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ParseLayerFlag parses a layer string from command-line flag (case-insensitive).
func ParseLayerFlag(s string) (log.Layer, error) {
	return parseLayer(s)
}

// parseLayer parses a layer string (case-insensitive).
func parseLayer(s string) (log.Layer, error) {
	switch strings.ToLower(s) {
	case "transport":
		return log.LayerTransport, nil
	case "wire":
		return log.LayerWire, nil
	case "service":
		return log.LayerService, nil
	default:
		return 0, fmt.Errorf("invalid layer: %s (must be transport, wire, or service)", s)
	}
}

// ParseDirectionFlag parses a direction string from command-line flag (case-insensitive).
func ParseDirectionFlag(s string) (log.Direction, error) {
	return parseDirection(s)
}

// parseDirection parses a direction string (case-insensitive).
func parseDirection(s string) (log.Direction, error) {
	switch strings.ToLower(s) {
	case "in":
		return log.DirectionIn, nil
	case "out":
		return log.DirectionOut, nil
	default:
		return 0, fmt.Errorf("invalid direction: %s (must be in or out)", s)
	}
}

// ParseCategoryFlag parses a category string from command-line flag (case-insensitive).
func ParseCategoryFlag(s string) (log.Category, error) {
	return parseCategory(s)
}

// parseCategory parses a category string (case-insensitive).
func parseCategory(s string) (log.Category, error) {
	switch strings.ToLower(s) {
	case "message":
		return log.CategoryMessage, nil
	case "control":
		return log.CategoryControl, nil
	case "state":
		return log.CategoryState, nil
	case "error":
		return log.CategoryError, nil
	case "snapshot":
		return log.CategorySnapshot, nil
	default:
		return 0, fmt.Errorf("invalid category: %s (must be message, control, state, error, or snapshot)", s)
	}
}

// RunView executes the view command.
func RunView(path string, filter ViewFilter, output io.Writer) error {
	reader, err := log.NewReader(path)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer reader.Close()

	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}

		// Apply filter
		if filter.Layer != nil && event.Layer != *filter.Layer {
			continue
		}
		if filter.Direction != nil && event.Direction != *filter.Direction {
			continue
		}
		if filter.Category != nil && event.Category != *filter.Category {
			continue
		}

		formatEvent(output, event)
	}

	return nil
}
