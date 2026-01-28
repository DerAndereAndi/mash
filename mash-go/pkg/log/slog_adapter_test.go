package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

func TestSlogAdapterLogsFrameEvent(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slogger := slog.New(handler)

	adapter := NewSlogAdapter(slogger)

	adapter.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
		Frame: &FrameEvent{
			Size: 256,
			Data: []byte{0x01, 0x02},
		},
	})

	output := buf.String()
	if output == "" {
		t.Fatal("no output produced")
	}

	// Parse JSON log entry
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	// Verify key fields
	if logEntry["conn_id"] != "conn-123" {
		t.Errorf("conn_id: got %v, want %q", logEntry["conn_id"], "conn-123")
	}
	if logEntry["direction"] != "IN" {
		t.Errorf("direction: got %v, want %q", logEntry["direction"], "IN")
	}
	if logEntry["layer"] != "TRANSPORT" {
		t.Errorf("layer: got %v, want %q", logEntry["layer"], "TRANSPORT")
	}
	if logEntry["frame_size"] != float64(256) {
		t.Errorf("frame_size: got %v, want %v", logEntry["frame_size"], 256)
	}
}

func TestSlogAdapterLogsMessageEvent(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slogger := slog.New(handler)

	adapter := NewSlogAdapter(slogger)

	op := wire.OpRead
	endpoint := uint8(1)
	feature := uint8(3)

	adapter.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-456",
		Direction:    DirectionOut,
		Layer:        LayerWire,
		Category:     CategoryMessage,
		Message: &MessageEvent{
			Type:       MessageTypeRequest,
			MessageID:  42,
			Operation:  &op,
			EndpointID: &endpoint,
			FeatureID:  &feature,
		},
	})

	output := buf.String()
	if output == "" {
		t.Fatal("no output produced")
	}

	// Parse JSON log entry
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	// Verify message fields
	if logEntry["msg_id"] != float64(42) {
		t.Errorf("msg_id: got %v, want %v", logEntry["msg_id"], 42)
	}
	if logEntry["msg_type"] != "REQUEST" {
		t.Errorf("msg_type: got %v, want %q", logEntry["msg_type"], "REQUEST")
	}
	if logEntry["operation"] != "Read" {
		t.Errorf("operation: got %v, want %q", logEntry["operation"], "Read")
	}
}

func TestSlogAdapterIncludesConnectionID(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slogger := slog.New(handler)

	adapter := NewSlogAdapter(slogger)

	adapter.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "abc12345-def6-7890",
		Direction:    DirectionIn,
		Layer:        LayerService,
		Category:     CategoryState,
		StateChange: &StateChangeEvent{
			Entity:   StateEntityConnection,
			NewState: "connected",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "abc12345-def6-7890") {
		t.Error("output does not contain connection ID")
	}
}

func TestSlogAdapterInterfaceSatisfaction(t *testing.T) {
	var _ Logger = (*SlogAdapter)(nil)
}
