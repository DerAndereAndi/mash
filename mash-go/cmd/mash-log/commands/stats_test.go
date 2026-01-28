package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
)

func TestStatsCountsByLayer(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, Layer: log.LayerTransport, Category: log.CategoryMessage},
		{Timestamp: ts, Layer: log.LayerTransport, Category: log.CategoryMessage},
		{Timestamp: ts, Layer: log.LayerWire, Category: log.CategoryMessage},
		{Timestamp: ts, Layer: log.LayerService, Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	// Check layer counts
	if !strings.Contains(output, "TRANSPORT:") {
		t.Error("expected TRANSPORT layer in output")
	}
	if !strings.Contains(output, "WIRE:") {
		t.Error("expected WIRE layer in output")
	}
	if !strings.Contains(output, "SERVICE:") {
		t.Error("expected SERVICE layer in output")
	}
}

func TestStatsCountsByCategory(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, Category: log.CategoryMessage},
		{Timestamp: ts, Category: log.CategoryControl},
		{Timestamp: ts, Category: log.CategoryState},
		{Timestamp: ts, Category: log.CategoryError, Error: &log.ErrorEventData{Message: "test"}},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	// Check category counts
	if !strings.Contains(output, "MESSAGE:") {
		t.Error("expected MESSAGE category in output")
	}
	if !strings.Contains(output, "CONTROL:") {
		t.Error("expected CONTROL category in output")
	}
	if !strings.Contains(output, "STATE:") {
		t.Error("expected STATE category in output")
	}
	if !strings.Contains(output, "ERROR:") {
		t.Error("expected ERROR category in output")
	}
}

func TestStatsCountsConnections(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, ConnectionID: "conn-aaaa-bbbb", Category: log.CategoryMessage},
		{Timestamp: ts.Add(time.Second), ConnectionID: "conn-aaaa-bbbb", Category: log.CategoryMessage},
		{Timestamp: ts, ConnectionID: "conn-cccc-dddd", Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	// Check connection count
	if !strings.Contains(output, "Connections: 2") {
		t.Errorf("expected 2 connections in output, got:\n%s", output)
	}

	// Check connection details
	if !strings.Contains(output, "[conn-aaa") {
		t.Error("expected conn-aaaa connection details")
	}
}

func TestStatsTotalEvents(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, Category: log.CategoryMessage},
		{Timestamp: ts, Category: log.CategoryMessage},
		{Timestamp: ts, Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Total Events: 3") {
		t.Errorf("expected 3 total events in output, got:\n%s", output)
	}
}

func TestStatsTimeRange(t *testing.T) {
	start := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 28, 11, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: start, Category: log.CategoryMessage},
		{Timestamp: end, Category: log.CategoryMessage},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Duration:") {
		t.Error("expected Duration in output")
	}
	if !strings.Contains(output, "1h0m0s") {
		t.Errorf("expected 1h0m0s duration in output, got:\n%s", output)
	}
}

func TestStatsErrorCount(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)
	events := []log.Event{
		{Timestamp: ts, Category: log.CategoryMessage},
		{Timestamp: ts, Category: log.CategoryError, Error: &log.ErrorEventData{Message: "error 1"}},
		{Timestamp: ts, Category: log.CategoryError, Error: &log.ErrorEventData{Message: "error 2"}},
	}

	path := createTestLogFile(t, events)

	var buf bytes.Buffer
	err := RunStats(path, &buf)
	if err != nil {
		t.Fatalf("RunStats failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Errors: 2") {
		t.Errorf("expected 2 errors in output, got:\n%s", output)
	}
}
