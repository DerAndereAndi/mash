package commands

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
)

// Stats holds aggregate statistics about a log file.
type Stats struct {
	TotalEvents     int
	EventsByLayer   map[log.Layer]int
	EventsByCategory map[log.Category]int
	EventsByDirection map[log.Direction]int
	Connections     map[string]*ConnectionStats
	Errors          int
	TimeRange       struct {
		Start time.Time
		End   time.Time
	}
}

// ConnectionStats holds statistics for a single connection.
type ConnectionStats struct {
	FirstSeen      time.Time
	LastSeen       time.Time
	Events         int
	DeviceID       string
	ZoneID         string
	SnapshotCount  int
	LastSnapshotAt time.Time
}

// RunStats analyzes the log file and prints statistics.
func RunStats(path string, w io.Writer) error {
	reader, err := log.NewReader(path)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer reader.Close()

	stats := &Stats{
		EventsByLayer:     make(map[log.Layer]int),
		EventsByCategory:  make(map[log.Category]int),
		EventsByDirection: make(map[log.Direction]int),
		Connections:       make(map[string]*ConnectionStats),
	}

	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}

		stats.TotalEvents++
		stats.EventsByLayer[event.Layer]++
		stats.EventsByCategory[event.Category]++
		stats.EventsByDirection[event.Direction]++

		// Track time range
		if stats.TimeRange.Start.IsZero() || event.Timestamp.Before(stats.TimeRange.Start) {
			stats.TimeRange.Start = event.Timestamp
		}
		if event.Timestamp.After(stats.TimeRange.End) {
			stats.TimeRange.End = event.Timestamp
		}

		// Track connection stats
		conn, ok := stats.Connections[event.ConnectionID]
		if !ok {
			conn = &ConnectionStats{
				FirstSeen: event.Timestamp,
				LastSeen:  event.Timestamp,
			}
			stats.Connections[event.ConnectionID] = conn
		}
		conn.Events++
		if event.Timestamp.After(conn.LastSeen) {
			conn.LastSeen = event.Timestamp
		}
		if event.DeviceID != "" && conn.DeviceID == "" {
			conn.DeviceID = event.DeviceID
		}
		if event.ZoneID != "" && conn.ZoneID == "" {
			conn.ZoneID = event.ZoneID
		}

		// Count snapshots per connection
		if event.Snapshot != nil {
			conn.SnapshotCount++
			if event.Timestamp.After(conn.LastSnapshotAt) {
				conn.LastSnapshotAt = event.Timestamp
			}
		}

		// Count errors
		if event.Error != nil {
			stats.Errors++
		}
	}

	printStats(w, stats)
	return nil
}

func printStats(w io.Writer, stats *Stats) {
	fmt.Fprintln(w, "=== MASH Protocol Log Statistics ===")
	fmt.Fprintln(w)

	// Time range
	if stats.TotalEvents > 0 {
		fmt.Fprintf(w, "Time Range: %s to %s\n",
			stats.TimeRange.Start.Format(time.RFC3339),
			stats.TimeRange.End.Format(time.RFC3339))
		fmt.Fprintf(w, "Duration:   %s\n", stats.TimeRange.End.Sub(stats.TimeRange.Start).Round(time.Second))
		fmt.Fprintln(w)
	}

	// Total events
	fmt.Fprintf(w, "Total Events: %d\n", stats.TotalEvents)
	fmt.Fprintln(w)

	// Events by layer
	fmt.Fprintln(w, "Events by Layer:")
	for _, layer := range []log.Layer{log.LayerTransport, log.LayerWire, log.LayerService} {
		if count := stats.EventsByLayer[layer]; count > 0 {
			fmt.Fprintf(w, "  %-12s %d\n", layer.String()+":", count)
		}
	}
	fmt.Fprintln(w)

	// Events by category
	fmt.Fprintln(w, "Events by Category:")
	for _, cat := range []log.Category{log.CategoryMessage, log.CategoryControl, log.CategoryState, log.CategoryError, log.CategorySnapshot} {
		if count := stats.EventsByCategory[cat]; count > 0 {
			fmt.Fprintf(w, "  %-12s %d\n", cat.String()+":", count)
		}
	}
	fmt.Fprintln(w)

	// Events by direction
	fmt.Fprintln(w, "Events by Direction:")
	for _, dir := range []log.Direction{log.DirectionIn, log.DirectionOut} {
		if count := stats.EventsByDirection[dir]; count > 0 {
			fmt.Fprintf(w, "  %-12s %d\n", dir.String()+":", count)
		}
	}
	fmt.Fprintln(w)

	// Connections
	fmt.Fprintf(w, "Connections: %d\n", len(stats.Connections))
	if len(stats.Connections) > 0 {
		// Sort by first seen time
		type connInfo struct {
			id    string
			stats *ConnectionStats
		}
		conns := make([]connInfo, 0, len(stats.Connections))
		for id, cs := range stats.Connections {
			conns = append(conns, connInfo{id, cs})
		}
		sort.Slice(conns, func(i, j int) bool {
			return conns[i].stats.FirstSeen.Before(conns[j].stats.FirstSeen)
		})

		fmt.Fprintln(w, "")
		for _, c := range conns {
			duration := c.stats.LastSeen.Sub(c.stats.FirstSeen).Round(time.Millisecond)
			shortID := c.id
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			fmt.Fprintf(w, "  [%s] %d events, duration %s\n", shortID, c.stats.Events, duration)
			if c.stats.DeviceID != "" {
				fmt.Fprintf(w, "           Device: %s\n", c.stats.DeviceID)
			}
			if c.stats.ZoneID != "" {
				fmt.Fprintf(w, "           Zone: %s\n", c.stats.ZoneID)
			}
			if c.stats.SnapshotCount > 0 {
				fmt.Fprintf(w, "           Snapshots: %d (last: %s)\n",
					c.stats.SnapshotCount, c.stats.LastSnapshotAt.Format(time.RFC3339))
			}
		}
	}

	// Errors
	if stats.Errors > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Errors: %d\n", stats.Errors)
	}
}
