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

func TestFormatSnapshotEvent(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456000, time.UTC)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionOut,
		Layer:        log.LayerService,
		Category:     log.CategorySnapshot,
		Snapshot: &log.CapabilitySnapshotEvent{
			Local: &log.DeviceSnapshot{
				DeviceID:    "device-001",
				SpecVersion: "1.0",
				Endpoints: []log.EndpointSnapshot{
					{
						ID:   0,
						Type: 0x00, // DEVICE_ROOT
						Features: []log.FeatureSnapshot{
							{
								ID:            0x01, // DeviceInfo
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 3, 12, 20, 21},
							},
						},
					},
					{
						ID:    1,
						Type:  0x05, // EV_CHARGER
						Label: "Wallbox",
						Features: []log.FeatureSnapshot{
							{
								ID:            0x03, // Electrical
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 3, 4, 5, 6, 7, 8},
							},
							{
								ID:            0x04, // Measurement
								FeatureMap:    0x0003,
								AttributeList: []uint16{1, 2, 3, 4, 5, 6},
							},
						},
					},
				},
				UseCases: []log.UseCaseSnapshot{
					{EndpointID: 1, ID: 0x01, Major: 1, Minor: 0, Scenarios: 0x07},
				},
			},
			Remote: &log.DeviceSnapshot{
				DeviceID: "controller-001",
				Endpoints: []log.EndpointSnapshot{
					{
						ID:   0,
						Type: 0x00, // DEVICE_ROOT
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check header
	if !strings.Contains(output, "Snapshot") {
		t.Errorf("expected Snapshot in header, got: %s", output)
	}

	// Check local device ID
	if !strings.Contains(output, "device-001") {
		t.Errorf("expected device-001, got: %s", output)
	}

	// Check endpoint types
	if !strings.Contains(output, "DEVICE_ROOT") {
		t.Errorf("expected DEVICE_ROOT, got: %s", output)
	}
	if !strings.Contains(output, "EV_CHARGER") {
		t.Errorf("expected EV_CHARGER, got: %s", output)
	}

	// Check feature names
	if !strings.Contains(output, "DeviceInfo") {
		t.Errorf("expected DeviceInfo feature, got: %s", output)
	}
	if !strings.Contains(output, "Electrical") {
		t.Errorf("expected Electrical feature, got: %s", output)
	}

	// Check featureMap
	if !strings.Contains(output, "featureMap=0x0001") {
		t.Errorf("expected featureMap=0x0001, got: %s", output)
	}

	// Check remote section
	if !strings.Contains(output, "Remote:") {
		t.Errorf("expected Remote: section, got: %s", output)
	}
	if !strings.Contains(output, "controller-001") {
		t.Errorf("expected controller-001, got: %s", output)
	}
}

func TestFormatSnapshotEvent_NoRemote(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456000, time.UTC)
	event := log.Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-6789-0123-4567-890abcdef012",
		Direction:    log.DirectionOut,
		Layer:        log.LayerService,
		Category:     log.CategorySnapshot,
		Snapshot: &log.CapabilitySnapshotEvent{
			Local: &log.DeviceSnapshot{
				DeviceID: "device-002",
				Endpoints: []log.EndpointSnapshot{
					{ID: 0, Type: 0x00},
				},
			},
		},
	}

	var buf bytes.Buffer
	formatEvent(&buf, event)
	output := buf.String()

	// Check local section appears
	if !strings.Contains(output, "Local:") {
		t.Errorf("expected Local: section, got: %s", output)
	}
	if !strings.Contains(output, "device-002") {
		t.Errorf("expected device-002, got: %s", output)
	}

	// Remote section should NOT appear
	if strings.Contains(output, "Remote:") {
		t.Errorf("expected no Remote: section, got: %s", output)
	}
}

func TestParseCategoryFlag_Snapshot(t *testing.T) {
	got, err := parseCategory("snapshot")
	if err != nil {
		t.Fatalf("parseCategory(%q) returned error: %v", "snapshot", err)
	}
	if got != log.CategorySnapshot {
		t.Errorf("parseCategory(%q) = %v, want %v", "snapshot", got, log.CategorySnapshot)
	}

	// Case-insensitive
	got, err = parseCategory("SNAPSHOT")
	if err != nil {
		t.Fatalf("parseCategory(%q) returned error: %v", "SNAPSHOT", err)
	}
	if got != log.CategorySnapshot {
		t.Errorf("parseCategory(%q) = %v, want %v", "SNAPSHOT", got, log.CategorySnapshot)
	}
}

func TestFilterBySnapshotCategory(t *testing.T) {
	events := []log.Event{
		{Category: log.CategoryMessage},
		{Category: log.CategorySnapshot, Snapshot: &log.CapabilitySnapshotEvent{}},
		{Category: log.CategoryState},
		{Category: log.CategorySnapshot, Snapshot: &log.CapabilitySnapshotEvent{}},
	}

	cat := log.CategorySnapshot
	filter := ViewFilter{Category: &cat}

	filtered := filterEvents(events, filter)
	if len(filtered) != 2 {
		t.Errorf("expected 2 snapshot events, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.Category != log.CategorySnapshot {
			t.Errorf("expected snapshot category, got %v", e.Category)
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
		{"snapshot", log.CategorySnapshot, false},
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
