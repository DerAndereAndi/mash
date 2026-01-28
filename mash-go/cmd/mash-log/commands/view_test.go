package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

func TestFormatFrameEvent(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456000, time.UTC)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionOut,
		Layer:        log.LayerTransport,
		Category:     log.CategoryMessage,
		Frame: &log.FrameEvent{
			Size:      128,
			Data:      []byte{0xa1, 0x01, 0x02, 0x03},
			Truncated: false,
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check timestamp format
	if !strings.Contains(output, "2026-01-28T10:15:32.123456Z") {
		t.Errorf("expected RFC3339Nano timestamp, got: %s", output)
	}

	// Check connection ID (shortened)
	if !strings.Contains(output, "[conn:abc12345]") {
		t.Errorf("expected shortened connection ID, got: %s", output)
	}

	// Check direction
	if !strings.Contains(output, "OUT") {
		t.Errorf("expected OUT direction, got: %s", output)
	}

	// Check layer
	if !strings.Contains(output, "TRANSPORT") {
		t.Errorf("expected TRANSPORT layer, got: %s", output)
	}

	// Check frame info
	if !strings.Contains(output, "Frame") {
		t.Errorf("expected Frame label, got: %s", output)
	}
	if !strings.Contains(output, "128 bytes") {
		t.Errorf("expected frame size, got: %s", output)
	}
}

func TestFormatMessageEventRequest(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456000, time.UTC)
	op := wire.OpRead
	endpoint := uint8(1)
	feature := uint8(3)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionOut,
		Layer:        log.LayerWire,
		Category:     log.CategoryMessage,
		Message: &log.MessageEvent{
			Type:       log.MessageTypeRequest,
			MessageID:  42,
			Operation:  &op,
			EndpointID: &endpoint,
			FeatureID:  &feature,
			Payload:    map[string]any{"attributeIds": []any{1, 2, 5}},
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check message type
	if !strings.Contains(output, "REQUEST") {
		t.Errorf("expected REQUEST type, got: %s", output)
	}

	// Check message ID
	if !strings.Contains(output, "MessageID: 42") {
		t.Errorf("expected MessageID: 42, got: %s", output)
	}

	// Check operation
	if !strings.Contains(output, "Operation: Read") {
		t.Errorf("expected Operation: Read, got: %s", output)
	}

	// Check endpoint/feature
	if !strings.Contains(output, "Endpoint: 1") {
		t.Errorf("expected Endpoint: 1, got: %s", output)
	}
	if !strings.Contains(output, "Feature: 3") {
		t.Errorf("expected Feature: 3, got: %s", output)
	}
}

func TestFormatMessageEventResponse(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 125789000, time.UTC)
	status := wire.StatusSuccess
	processingTime := 2333 * time.Microsecond
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionIn,
		Layer:        log.LayerWire,
		Category:     log.CategoryMessage,
		Message: &log.MessageEvent{
			Type:           log.MessageTypeResponse,
			MessageID:      42,
			Status:         &status,
			ProcessingTime: &processingTime,
			Payload:        map[string]any{"1": 3500, "2": 230, "5": 15.2},
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check message type
	if !strings.Contains(output, "RESPONSE") {
		t.Errorf("expected RESPONSE type, got: %s", output)
	}

	// Check status
	if !strings.Contains(output, "Status: SUCCESS") {
		t.Errorf("expected Status: SUCCESS, got: %s", output)
	}

	// Check duration
	if !strings.Contains(output, "Duration:") {
		t.Errorf("expected Duration, got: %s", output)
	}
}

func TestFormatStateChangeEvent(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 30, 0, time.UTC)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionIn,
		Layer:        log.LayerService,
		Category:     log.CategoryState,
		StateChange: &log.StateChangeEvent{
			Entity:   log.StateEntityConnection,
			OldState: "",
			NewState: "connected",
			Reason:   "TLS handshake complete",
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check category
	if !strings.Contains(output, "State") {
		t.Errorf("expected State category, got: %s", output)
	}

	// Check entity
	if !strings.Contains(output, "CONNECTION") {
		t.Errorf("expected CONNECTION entity, got: %s", output)
	}

	// Check new state
	if !strings.Contains(output, "connected") {
		t.Errorf("expected connected state, got: %s", output)
	}
}

func TestFormatControlMsgEvent(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 35, 0, time.UTC)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionOut,
		Layer:        log.LayerTransport,
		Category:     log.CategoryControl,
		ControlMsg: &log.ControlMsgEvent{
			Type: log.ControlMsgPing,
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check control message type
	if !strings.Contains(output, "CTRL") {
		t.Errorf("expected CTRL category, got: %s", output)
	}
	if !strings.Contains(output, "PING") {
		t.Errorf("expected PING type, got: %s", output)
	}
}

func TestFilterByLayer(t *testing.T) {
	events := []log.Event{
		{Layer: log.LayerTransport, Category: log.CategoryMessage},
		{Layer: log.LayerWire, Category: log.CategoryMessage},
		{Layer: log.LayerService, Category: log.CategoryMessage},
	}

	wire := log.LayerWire
	filter := ViewFilter{Layer: &wire}

	filtered := filterEvents(events, filter)
	if len(filtered) != 1 {
		t.Errorf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].Layer != log.LayerWire {
		t.Errorf("expected wire layer, got %v", filtered[0].Layer)
	}
}

func TestFilterByDirection(t *testing.T) {
	events := []log.Event{
		{Direction: log.DirectionIn, Category: log.CategoryMessage},
		{Direction: log.DirectionOut, Category: log.CategoryMessage},
		{Direction: log.DirectionIn, Category: log.CategoryMessage},
	}

	out := log.DirectionOut
	filter := ViewFilter{Direction: &out}

	filtered := filterEvents(events, filter)
	if len(filtered) != 1 {
		t.Errorf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].Direction != log.DirectionOut {
		t.Errorf("expected out direction, got %v", filtered[0].Direction)
	}
}

func TestFilterByCategory(t *testing.T) {
	events := []log.Event{
		{Category: log.CategoryMessage},
		{Category: log.CategoryControl},
		{Category: log.CategoryState},
		{Category: log.CategoryError},
	}

	state := log.CategoryState
	filter := ViewFilter{Category: &state}

	filtered := filterEvents(events, filter)
	if len(filtered) != 1 {
		t.Errorf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].Category != log.CategoryState {
		t.Errorf("expected state category, got %v", filtered[0].Category)
	}
}

func TestParseLayer(t *testing.T) {
	tests := []struct {
		input    string
		expected log.Layer
		wantErr  bool
	}{
		{"transport", log.LayerTransport, false},
		{"TRANSPORT", log.LayerTransport, false},
		{"wire", log.LayerWire, false},
		{"service", log.LayerService, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := parseLayer(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseLayer(%q) expected error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseLayer(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("parseLayer(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		}
	}
}

func TestParseDirection(t *testing.T) {
	tests := []struct {
		input    string
		expected log.Direction
		wantErr  bool
	}{
		{"in", log.DirectionIn, false},
		{"IN", log.DirectionIn, false},
		{"out", log.DirectionOut, false},
		{"OUT", log.DirectionOut, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := parseDirection(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseDirection(%q) expected error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseDirection(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("parseDirection(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		}
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected log.Category
		wantErr  bool
	}{
		{"message", log.CategoryMessage, false},
		{"MESSAGE", log.CategoryMessage, false},
		{"control", log.CategoryControl, false},
		{"state", log.CategoryState, false},
		{"error", log.CategoryError, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := parseCategory(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseCategory(%q) expected error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseCategory(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("parseCategory(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		}
	}
}
