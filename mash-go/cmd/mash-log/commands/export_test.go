package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

func createTestLogFile(t *testing.T, events []log.Event) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := log.NewFileLogger(path)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	for _, e := range events {
		logger.Log(e)
	}
	logger.Close()

	return path
}

func TestExportToJSONL(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456000, time.UTC)
	op := wire.OpRead
	events := []log.Event{
		{
			Timestamp:    ts,
			ConnectionID: "abc12345",
			Direction:    log.DirectionOut,
			Layer:        log.LayerWire,
			Category:     log.CategoryMessage,
			Message: &log.MessageEvent{
				Type:      log.MessageTypeRequest,
				MessageID: 42,
				Operation: &op,
			},
		},
		{
			Timestamp:    ts.Add(time.Second),
			ConnectionID: "abc12345",
			Direction:    log.DirectionIn,
			Layer:        log.LayerWire,
			Category:     log.CategoryMessage,
			Message: &log.MessageEvent{
				Type:      log.MessageTypeResponse,
				MessageID: 42,
			},
		},
	}

	path := createTestLogFile(t, events)

	// Export to JSONL in memory (via temp file)
	outPath := filepath.Join(t.TempDir(), "out.jsonl")
	err := RunExport(path, "jsonl", outPath)
	if err != nil {
		t.Fatalf("RunExport failed: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Parse first line
	var event1 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event1); err != nil {
		t.Errorf("failed to parse line 1: %v", err)
	}
	if event1["ConnectionID"] != "abc12345" {
		t.Errorf("expected ConnectionID abc12345, got %v", event1["ConnectionID"])
	}
}

func TestExportToCSV(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 0, time.UTC)
	events := []log.Event{
		{
			Timestamp:    ts,
			ConnectionID: "abc12345",
			Direction:    log.DirectionOut,
			Layer:        log.LayerTransport,
			Category:     log.CategoryMessage,
			Frame: &log.FrameEvent{
				Size: 64,
				Data: []byte{0x01, 0x02},
			},
		},
	}

	path := createTestLogFile(t, events)

	outPath := filepath.Join(t.TempDir(), "out.csv")
	err := RunExport(path, "csv", outPath)
	if err != nil {
		t.Fatalf("RunExport failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Check header
	if !strings.HasPrefix(string(data), "timestamp,connection_id,direction,layer,category") {
		t.Errorf("expected CSV header, got: %s", string(data[:50]))
	}

	// Check data row exists
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		t.Errorf("expected header + data row, got %d lines", len(lines))
	}
}

func TestExportWritesToStdout(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 0, time.UTC)
	events := []log.Event{
		{
			Timestamp:    ts,
			ConnectionID: "abc12345",
			Direction:    log.DirectionOut,
			Layer:        log.LayerTransport,
			Category:     log.CategoryMessage,
			Frame:        &log.FrameEvent{Size: 64},
		},
	}

	path := createTestLogFile(t, events)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunExport(path, "jsonl", "") // empty output means stdout

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunExport failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() == 0 {
		t.Error("expected output to stdout")
	}
}

func TestExportUnknownFormat(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 0, time.UTC)
	events := []log.Event{
		{
			Timestamp:    ts,
			ConnectionID: "abc12345",
			Frame:        &log.FrameEvent{Size: 64},
		},
	}

	path := createTestLogFile(t, events)
	outPath := filepath.Join(t.TempDir(), "out.xml")

	err := RunExport(path, "xml", outPath)
	if err == nil {
		t.Error("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected 'unknown format' error, got: %v", err)
	}
}
