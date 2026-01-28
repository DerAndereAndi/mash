package log

import (
	"io"
	"path/filepath"
	"testing"
	"time"
)

func createTestLogFile(t *testing.T, events []Event) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("failed to create test log: %v", err)
	}

	for _, e := range events {
		logger.Log(e)
	}
	logger.Close()

	return path
}

func TestReaderIteratesEvents(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-1", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-2", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-3", Direction: DirectionIn, Layer: LayerService, Category: CategoryState},
	}

	path := createTestLogFile(t, events)

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	if len(read) != 3 {
		t.Fatalf("got %d events, want 3", len(read))
	}

	// Verify order
	if read[0].ConnectionID != "conn-1" {
		t.Errorf("first event ConnectionID = %q, want %q", read[0].ConnectionID, "conn-1")
	}
	if read[2].ConnectionID != "conn-3" {
		t.Errorf("last event ConnectionID = %q, want %q", read[2].ConnectionID, "conn-3")
	}
}

func TestReaderHandlesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.mlog")

	// Create empty file
	logger, _ := NewFileLogger(path)
	logger.Close()

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	event, err := reader.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got err=%v, event=%+v", err, event)
	}
}

func TestReaderHandlesTruncatedFile(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-1", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
	}

	path := createTestLogFile(t, events)

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	// Read first event
	_, err = reader.Next()
	if err != nil {
		t.Fatalf("first Next failed: %v", err)
	}

	// Second read should return EOF
	_, err = reader.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF after all events, got %v", err)
	}
}

func TestReaderFilterByConnectionID(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-A", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-B", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-A", Direction: DirectionIn, Layer: LayerService, Category: CategoryState},
		{Timestamp: time.Now(), ConnectionID: "conn-C", Direction: DirectionOut, Layer: LayerTransport, Category: CategoryMessage},
	}

	path := createTestLogFile(t, events)

	filter := Filter{ConnectionID: "conn-A"}
	reader, err := NewFilteredReader(path, filter)
	if err != nil {
		t.Fatalf("NewFilteredReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	if len(read) != 2 {
		t.Fatalf("got %d events, want 2", len(read))
	}

	for _, e := range read {
		if e.ConnectionID != "conn-A" {
			t.Errorf("event has ConnectionID=%q, want %q", e.ConnectionID, "conn-A")
		}
	}
}

func TestReaderFilterByLayer(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-1", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-2", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-3", Direction: DirectionIn, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-4", Direction: DirectionOut, Layer: LayerService, Category: CategoryState},
	}

	path := createTestLogFile(t, events)

	layer := LayerWire
	filter := Filter{Layer: &layer}
	reader, err := NewFilteredReader(path, filter)
	if err != nil {
		t.Fatalf("NewFilteredReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	if len(read) != 2 {
		t.Fatalf("got %d events, want 2", len(read))
	}

	for _, e := range read {
		if e.Layer != LayerWire {
			t.Errorf("event has Layer=%v, want %v", e.Layer, LayerWire)
		}
	}
}

func TestReaderFilterByTimeRange(t *testing.T) {
	baseTime := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)

	events := []Event{
		{Timestamp: baseTime.Add(-1 * time.Hour), ConnectionID: "conn-1", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: baseTime, ConnectionID: "conn-2", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: baseTime.Add(30 * time.Minute), ConnectionID: "conn-3", Direction: DirectionIn, Layer: LayerService, Category: CategoryState},
		{Timestamp: baseTime.Add(2 * time.Hour), ConnectionID: "conn-4", Direction: DirectionOut, Layer: LayerTransport, Category: CategoryMessage},
	}

	path := createTestLogFile(t, events)

	start := baseTime.Add(-5 * time.Minute)
	end := baseTime.Add(1 * time.Hour)
	filter := Filter{
		TimeStart: &start,
		TimeEnd:   &end,
	}
	reader, err := NewFilteredReader(path, filter)
	if err != nil {
		t.Fatalf("NewFilteredReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	if len(read) != 2 {
		t.Fatalf("got %d events, want 2 (events within time range)", len(read))
	}

	// Verify it's the middle two events
	if read[0].ConnectionID != "conn-2" {
		t.Errorf("first event ConnectionID = %q, want %q", read[0].ConnectionID, "conn-2")
	}
	if read[1].ConnectionID != "conn-3" {
		t.Errorf("second event ConnectionID = %q, want %q", read[1].ConnectionID, "conn-3")
	}
}

func TestReaderFilterByDirection(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-1", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-2", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-3", Direction: DirectionIn, Layer: LayerService, Category: CategoryState},
		{Timestamp: time.Now(), ConnectionID: "conn-4", Direction: DirectionOut, Layer: LayerTransport, Category: CategoryControl},
	}

	path := createTestLogFile(t, events)

	dir := DirectionOut
	filter := Filter{Direction: &dir}
	reader, err := NewFilteredReader(path, filter)
	if err != nil {
		t.Fatalf("NewFilteredReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	if len(read) != 2 {
		t.Fatalf("got %d events, want 2", len(read))
	}

	for _, e := range read {
		if e.Direction != DirectionOut {
			t.Errorf("event has Direction=%v, want %v", e.Direction, DirectionOut)
		}
	}
}

func TestReaderCombinedFilters(t *testing.T) {
	events := []Event{
		{Timestamp: time.Now(), ConnectionID: "conn-A", Direction: DirectionIn, Layer: LayerTransport, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-A", Direction: DirectionOut, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-B", Direction: DirectionIn, Layer: LayerWire, Category: CategoryMessage},
		{Timestamp: time.Now(), ConnectionID: "conn-A", Direction: DirectionIn, Layer: LayerWire, Category: CategoryMessage},
	}

	path := createTestLogFile(t, events)

	layer := LayerWire
	dir := DirectionIn
	filter := Filter{
		ConnectionID: "conn-A",
		Layer:        &layer,
		Direction:    &dir,
	}
	reader, err := NewFilteredReader(path, filter)
	if err != nil {
		t.Fatalf("NewFilteredReader failed: %v", err)
	}
	defer reader.Close()

	var read []Event
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		read = append(read, event)
	}

	// Only the last event matches all criteria
	if len(read) != 1 {
		t.Fatalf("got %d events, want 1", len(read))
	}

	if read[0].ConnectionID != "conn-A" || read[0].Layer != LayerWire || read[0].Direction != DirectionIn {
		t.Error("event doesn't match all filter criteria")
	}
}
