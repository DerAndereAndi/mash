package log

import (
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"
)

// logEncMode is the CBOR encoder mode for log events.
// Configured for nanosecond-precision timestamps and deterministic encoding.
var logEncMode cbor.EncMode

// logDecMode is the CBOR decoder mode for log events.
var logDecMode cbor.DecMode

func init() {
	var err error

	// Configure encoder for log events
	// Uses RFC3339Nano for nanosecond-precision timestamps
	encOpts := cbor.EncOptions{
		Sort:          cbor.SortCanonical,
		IndefLength:   cbor.IndefLengthForbidden,
		NilContainers: cbor.NilContainerAsNull,
		Time:          cbor.TimeRFC3339Nano, // Nanosecond precision
	}
	logEncMode, err = encOpts.EncMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create log CBOR encoder mode: %v", err))
	}

	// Configure decoder for log events
	decOpts := cbor.DecOptions{
		DupMapKey:         cbor.DupMapKeyQuiet,
		IndefLength:       cbor.IndefLengthAllowed,
		ExtraReturnErrors: cbor.ExtraDecErrorNone,
	}
	logDecMode, err = decOpts.DecMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create log CBOR decoder mode: %v", err))
	}
}

// EncodeEvent encodes an Event to CBOR bytes using integer keys for compactness.
func EncodeEvent(event Event) ([]byte, error) {
	return logEncMode.Marshal(event)
}

// DecodeEvent decodes CBOR bytes into an Event.
func DecodeEvent(data []byte) (Event, error) {
	var event Event
	if err := logDecMode.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}
	return event, nil
}

// NewEncoder creates a CBOR encoder for log events that writes to w.
func NewEncoder(w io.Writer) *cbor.Encoder {
	return logEncMode.NewEncoder(w)
}

// NewDecoder creates a CBOR decoder for log events that reads from r.
func NewDecoder(r io.Reader) *cbor.Decoder {
	return logDecMode.NewDecoder(r)
}
