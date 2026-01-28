package commands

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
)

func TestFilterByConnectionID(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, ConnectionID: "conn-1", Category: log.CategoryMessage},
		{Timestamp: ts, ConnectionID: "conn-2", Category: log.CategoryMessage},
		{Timestamp: ts, ConnectionID: "conn-1", Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)
	outPath := filepath.Join(t.TempDir(), "filtered.mlog")

	err := RunFilter(path, FilterOptions{
		Output: outPath,
		ConnID: "conn-1",
	})
	if err != nil {
		t.Fatalf("RunFilter failed: %v", err)
	}

	// Verify output
	reader, err := log.NewReader(outPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer reader.Close()

	count := 0
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read event: %v", err)
		}
		if event.ConnectionID != "conn-1" {
			t.Errorf("expected conn-1, got %s", event.ConnectionID)
		}
		count++
	}

	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

func TestFilterByTimeRange(t *testing.T) {
	base := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: base, ConnectionID: "conn-1", Category: log.CategoryMessage},
		{Timestamp: base.Add(time.Hour), ConnectionID: "conn-1", Category: log.CategoryMessage},
		{Timestamp: base.Add(2 * time.Hour), ConnectionID: "conn-1", Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)
	outPath := filepath.Join(t.TempDir(), "filtered.mlog")

	err := RunFilter(path, FilterOptions{
		Output:    outPath,
		TimeStart: base.Add(30 * time.Minute).Format(time.RFC3339),
		TimeEnd:   base.Add(90 * time.Minute).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("RunFilter failed: %v", err)
	}

	// Verify output - should only have the 10:00 + 1hr event
	reader, err := log.NewReader(outPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer reader.Close()

	count := 0
	for {
		_, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read event: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestFilterCommandByLayer(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, Layer: log.LayerTransport, Category: log.CategoryMessage},
		{Timestamp: ts, Layer: log.LayerWire, Category: log.CategoryMessage},
		{Timestamp: ts, Layer: log.LayerService, Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)
	outPath := filepath.Join(t.TempDir(), "filtered.mlog")

	err := RunFilter(path, FilterOptions{
		Output: outPath,
		Layer:  "wire",
	})
	if err != nil {
		t.Fatalf("RunFilter failed: %v", err)
	}

	reader, err := log.NewReader(outPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer reader.Close()

	count := 0
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read event: %v", err)
		}
		if event.Layer != log.LayerWire {
			t.Errorf("expected wire layer, got %v", event.Layer)
		}
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestFilterWritesCBOR(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, ConnectionID: "conn-1", Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)
	outPath := filepath.Join(t.TempDir(), "filtered.mlog")

	err := RunFilter(path, FilterOptions{
		Output: outPath,
	})
	if err != nil {
		t.Fatalf("RunFilter failed: %v", err)
	}

	// Verify it's readable as CBOR
	reader, err := log.NewReader(outPath)
	if err != nil {
		t.Fatalf("failed to open output as CBOR: %v", err)
	}
	defer reader.Close()

	event, err := reader.Next()
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

	if event.ConnectionID != "conn-1" {
		t.Errorf("expected conn-1, got %s", event.ConnectionID)
	}
}
