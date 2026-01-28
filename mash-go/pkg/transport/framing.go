package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
)

// Framing constants.
const (
	// LengthPrefixSize is the size of the length prefix in bytes.
	LengthPrefixSize = 4

	// DefaultMaxMessageSize is the default maximum message size (64 KB).
	DefaultMaxMessageSize = 65536

	// MinMessageSize is the minimum valid message size.
	MinMessageSize = 1

	// MaxLogFrameDataSize is the maximum frame data size to include in logs (4 KB).
	// Larger frames are truncated in log events to avoid excessive memory usage.
	MaxLogFrameDataSize = 4096
)

// Framing errors.
var (
	// ErrMessageTooLarge indicates the message exceeds the maximum size.
	ErrMessageTooLarge = errors.New("message too large")

	// ErrMessageEmpty indicates an empty message.
	ErrMessageEmpty = errors.New("message is empty")

	// ErrFrameTruncated indicates the frame was truncated.
	ErrFrameTruncated = errors.New("frame truncated")
)

// FrameWriter writes length-prefixed frames to an underlying writer.
type FrameWriter struct {
	w              io.Writer
	maxMessageSize uint32
	mu             sync.Mutex

	// Logging support (optional)
	logger log.Logger
	connID string
}

// NewFrameWriter creates a new frame writer.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{
		w:              w,
		maxMessageSize: DefaultMaxMessageSize,
	}
}

// NewFrameWriterWithMaxSize creates a frame writer with a custom max size.
func NewFrameWriterWithMaxSize(w io.Writer, maxSize uint32) *FrameWriter {
	return &FrameWriter{
		w:              w,
		maxMessageSize: maxSize,
	}
}

// SetLogger configures logging for this writer.
// Pass nil to disable logging.
func (fw *FrameWriter) SetLogger(logger log.Logger, connID string) {
	fw.logger = logger
	fw.connID = connID
}

// WriteFrame writes a length-prefixed frame.
// Thread-safe: can be called from multiple goroutines.
func (fw *FrameWriter) WriteFrame(data []byte) error {
	if len(data) == 0 {
		return ErrMessageEmpty
	}
	if uint32(len(data)) > fw.maxMessageSize {
		return fmt.Errorf("%w: %d > %d", ErrMessageTooLarge, len(data), fw.maxMessageSize)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Write length prefix (4 bytes, big-endian)
	var lengthBuf [LengthPrefixSize]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(data)))

	if _, err := fw.w.Write(lengthBuf[:]); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}

	// Write payload
	if _, err := fw.w.Write(data); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	// Log the frame if logger is configured
	if fw.logger != nil {
		fw.logger.Log(fw.makeFrameEvent(data, log.DirectionOut))
	}

	return nil
}

// makeFrameEvent creates a log event for a frame.
func (fw *FrameWriter) makeFrameEvent(data []byte, direction log.Direction) log.Event {
	frameSize := LengthPrefixSize + len(data)
	frameData := data
	truncated := false

	if len(data) > MaxLogFrameDataSize {
		frameData = data[:MaxLogFrameDataSize]
		truncated = true
	}

	return log.Event{
		Timestamp:    time.Now(),
		ConnectionID: fw.connID,
		Direction:    direction,
		Layer:        log.LayerTransport,
		Category:     log.CategoryMessage,
		Frame: &log.FrameEvent{
			Size:      frameSize,
			Data:      frameData,
			Truncated: truncated,
		},
	}
}

// FrameReader reads length-prefixed frames from an underlying reader.
type FrameReader struct {
	r              io.Reader
	maxMessageSize uint32
	lengthBuf      [LengthPrefixSize]byte

	// Logging support (optional)
	logger log.Logger
	connID string
}

// NewFrameReader creates a new frame reader.
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{
		r:              r,
		maxMessageSize: DefaultMaxMessageSize,
	}
}

// NewFrameReaderWithMaxSize creates a frame reader with a custom max size.
func NewFrameReaderWithMaxSize(r io.Reader, maxSize uint32) *FrameReader {
	return &FrameReader{
		r:              r,
		maxMessageSize: maxSize,
	}
}

// SetLogger configures logging for this reader.
// Pass nil to disable logging.
func (fr *FrameReader) SetLogger(logger log.Logger, connID string) {
	fr.logger = logger
	fr.connID = connID
}

// ReadFrame reads a length-prefixed frame.
// Returns the frame payload (without the length prefix).
func (fr *FrameReader) ReadFrame() ([]byte, error) {
	// Read length prefix
	if _, err := io.ReadFull(fr.r, fr.lengthBuf[:]); err != nil {
		if err == io.EOF {
			return nil, err
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrFrameTruncated
		}
		return nil, fmt.Errorf("failed to read length prefix: %w", err)
	}

	length := binary.BigEndian.Uint32(fr.lengthBuf[:])

	// Validate length
	if length == 0 {
		return nil, ErrMessageEmpty
	}
	if length > fr.maxMessageSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrMessageTooLarge, length, fr.maxMessageSize)
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(fr.r, payload); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || err == io.EOF {
			return nil, ErrFrameTruncated
		}
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	// Log the frame if logger is configured
	if fr.logger != nil {
		fr.logger.Log(fr.makeFrameEvent(payload, log.DirectionIn))
	}

	return payload, nil
}

// makeFrameEvent creates a log event for a frame.
func (fr *FrameReader) makeFrameEvent(data []byte, direction log.Direction) log.Event {
	frameSize := LengthPrefixSize + len(data)
	frameData := data
	truncated := false

	if len(data) > MaxLogFrameDataSize {
		frameData = data[:MaxLogFrameDataSize]
		truncated = true
	}

	return log.Event{
		Timestamp:    time.Now(),
		ConnectionID: fr.connID,
		Direction:    direction,
		Layer:        log.LayerTransport,
		Category:     log.CategoryMessage,
		Frame: &log.FrameEvent{
			Size:      frameSize,
			Data:      frameData,
			Truncated: truncated,
		},
	}
}

// SetMaxMessageSize updates the maximum message size.
func (fr *FrameReader) SetMaxMessageSize(size uint32) {
	fr.maxMessageSize = size
}

// Framer combines frame reading and writing.
type Framer struct {
	*FrameReader
	*FrameWriter
}

// NewFramer creates a new framer for bidirectional communication.
func NewFramer(rw io.ReadWriter) *Framer {
	return &Framer{
		FrameReader: NewFrameReader(rw),
		FrameWriter: NewFrameWriter(rw),
	}
}

// NewFramerWithMaxSize creates a framer with a custom max message size.
func NewFramerWithMaxSize(rw io.ReadWriter, maxSize uint32) *Framer {
	return &Framer{
		FrameReader: NewFrameReaderWithMaxSize(rw, maxSize),
		FrameWriter: NewFrameWriterWithMaxSize(rw, maxSize),
	}
}

// SetLogger configures logging for both reader and writer.
// Pass nil to disable logging.
func (f *Framer) SetLogger(logger log.Logger, connID string) {
	f.FrameReader.SetLogger(logger, connID)
	f.FrameWriter.SetLogger(logger, connID)
}

// FrameSize returns the total frame size including the length prefix.
func FrameSize(payloadSize int) int {
	return LengthPrefixSize + payloadSize
}
