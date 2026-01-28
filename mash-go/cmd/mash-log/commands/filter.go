package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
)

// FilterOptions specifies filtering criteria for the filter command.
type FilterOptions struct {
	Output    string
	ConnID    string
	DeviceID  string
	ZoneID    string
	TimeStart string
	TimeEnd   string
	Layer     string
	Direction string
	Category  string
}

// RunFilter filters the log file and writes matching events to a new file.
func RunFilter(path string, opts FilterOptions) error {
	// Build filter
	filter := log.Filter{
		ConnectionID: opts.ConnID,
		DeviceID:     opts.DeviceID,
		ZoneID:       opts.ZoneID,
	}

	if opts.TimeStart != "" {
		t, err := time.Parse(time.RFC3339, opts.TimeStart)
		if err != nil {
			return fmt.Errorf("invalid time-start format: %w", err)
		}
		filter.TimeStart = &t
	}

	if opts.TimeEnd != "" {
		t, err := time.Parse(time.RFC3339, opts.TimeEnd)
		if err != nil {
			return fmt.Errorf("invalid time-end format: %w", err)
		}
		filter.TimeEnd = &t
	}

	if opts.Layer != "" {
		l, err := parseLayer(opts.Layer)
		if err != nil {
			return err
		}
		filter.Layer = &l
	}

	if opts.Direction != "" {
		d, err := parseDirection(opts.Direction)
		if err != nil {
			return err
		}
		filter.Direction = &d
	}

	if opts.Category != "" {
		c, err := parseCategory(opts.Category)
		if err != nil {
			return err
		}
		filter.Category = &c
	}

	// Open input
	reader, err := log.NewFilteredReader(path, filter)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer reader.Close()

	// Create file logger to write filtered events
	logger, err := log.NewFileLogger(opts.Output)
	if err != nil {
		return fmt.Errorf("failed to create output logger: %w", err)
	}
	defer logger.Close()

	count := 0
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}

		logger.Log(event)
		count++
	}

	fmt.Printf("Filtered %d events to %s\n", count, opts.Output)
	return nil
}
