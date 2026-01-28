package log

import (
	"io"
	"os"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// Filter specifies criteria for filtering log events.
// Empty/nil fields match all events for that criterion.
type Filter struct {
	// ConnectionID filters by exact connection ID match.
	ConnectionID string

	// Direction filters by message direction.
	Direction *Direction

	// Layer filters by protocol layer.
	Layer *Layer

	// Category filters by event category.
	Category *Category

	// TimeStart filters events at or after this time.
	TimeStart *time.Time

	// TimeEnd filters events before this time.
	TimeEnd *time.Time

	// DeviceID filters by device ID.
	DeviceID string

	// ZoneID filters by zone ID.
	ZoneID string
}

// matches returns true if the event matches all filter criteria.
func (f *Filter) matches(event Event) bool {
	if f.ConnectionID != "" && event.ConnectionID != f.ConnectionID {
		return false
	}
	if f.Direction != nil && event.Direction != *f.Direction {
		return false
	}
	if f.Layer != nil && event.Layer != *f.Layer {
		return false
	}
	if f.Category != nil && event.Category != *f.Category {
		return false
	}
	if f.TimeStart != nil && event.Timestamp.Before(*f.TimeStart) {
		return false
	}
	if f.TimeEnd != nil && !event.Timestamp.Before(*f.TimeEnd) {
		return false
	}
	if f.DeviceID != "" && event.DeviceID != f.DeviceID {
		return false
	}
	if f.ZoneID != "" && event.ZoneID != f.ZoneID {
		return false
	}
	return true
}

// Reader reads protocol log events from a CBOR-encoded file.
// It provides an iterator interface for streaming large files.
type Reader struct {
	file    *os.File
	decoder *cbor.Decoder
	filter  Filter
}

// NewReader creates a Reader that reads all events from the specified log file.
func NewReader(path string) (*Reader, error) {
	return NewFilteredReader(path, Filter{})
}

// NewFilteredReader creates a Reader that reads events matching the filter.
func NewFilteredReader(path string, filter Filter) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{
		file:    f,
		decoder: NewDecoder(f),
		filter:  filter,
	}, nil
}

// Next returns the next event that matches the filter.
// Returns io.EOF when no more events are available.
func (r *Reader) Next() (Event, error) {
	for {
		var event Event
		if err := r.decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return Event{}, io.EOF
			}
			return Event{}, err
		}

		if r.filter.matches(event) {
			return event, nil
		}
		// Event doesn't match filter, continue to next
	}
}

// Close closes the underlying file.
func (r *Reader) Close() error {
	return r.file.Close()
}
