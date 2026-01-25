package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestFrameWriterReader(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "small message",
			payload: []byte("hello"),
		},
		{
			name:    "medium message",
			payload: bytes.Repeat([]byte("x"), 1000),
		},
		{
			name:    "max size message",
			payload: bytes.Repeat([]byte("y"), DefaultMaxMessageSize),
		},
		{
			name:    "single byte",
			payload: []byte{0x42},
		},
		{
			name:    "binary data",
			payload: []byte{0x00, 0xFF, 0x7F, 0x80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)

			// Write frame
			writer := NewFrameWriter(buf)
			if err := writer.WriteFrame(tt.payload); err != nil {
				t.Fatalf("WriteFrame failed: %v", err)
			}

			// Check frame size
			expectedSize := LengthPrefixSize + len(tt.payload)
			if buf.Len() != expectedSize {
				t.Errorf("frame size = %d, want %d", buf.Len(), expectedSize)
			}

			// Read frame
			reader := NewFrameReader(buf)
			got, err := reader.ReadFrame()
			if err != nil {
				t.Fatalf("ReadFrame failed: %v", err)
			}

			// Verify payload
			if !bytes.Equal(got, tt.payload) {
				t.Errorf("payload mismatch: got %d bytes, want %d bytes", len(got), len(tt.payload))
			}
		})
	}
}

func TestFrameWriterEmptyMessage(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewFrameWriter(buf)

	err := writer.WriteFrame([]byte{})
	if !errors.Is(err, ErrMessageEmpty) {
		t.Errorf("expected ErrMessageEmpty, got %v", err)
	}

	err = writer.WriteFrame(nil)
	if !errors.Is(err, ErrMessageEmpty) {
		t.Errorf("expected ErrMessageEmpty for nil, got %v", err)
	}
}

func TestFrameWriterMessageTooLarge(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewFrameWriterWithMaxSize(buf, 100)

	err := writer.WriteFrame(bytes.Repeat([]byte("x"), 101))
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestFrameReaderMessageTooLarge(t *testing.T) {
	buf := new(bytes.Buffer)

	// Write a frame with length > max
	var lengthBuf [LengthPrefixSize]byte
	binary.BigEndian.PutUint32(lengthBuf[:], 1000)
	buf.Write(lengthBuf[:])
	buf.Write(bytes.Repeat([]byte("x"), 1000))

	// Try to read with smaller max size
	reader := NewFrameReaderWithMaxSize(buf, 100)
	_, err := reader.ReadFrame()
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestFrameReaderEmptyLength(t *testing.T) {
	buf := new(bytes.Buffer)

	// Write frame with length = 0
	var lengthBuf [LengthPrefixSize]byte
	binary.BigEndian.PutUint32(lengthBuf[:], 0)
	buf.Write(lengthBuf[:])

	reader := NewFrameReader(buf)
	_, err := reader.ReadFrame()
	if !errors.Is(err, ErrMessageEmpty) {
		t.Errorf("expected ErrMessageEmpty, got %v", err)
	}
}

func TestFrameReaderTruncatedLength(t *testing.T) {
	buf := new(bytes.Buffer)

	// Write only 2 bytes of length prefix
	buf.Write([]byte{0x00, 0x01})

	reader := NewFrameReader(buf)
	_, err := reader.ReadFrame()
	if !errors.Is(err, ErrFrameTruncated) {
		t.Errorf("expected ErrFrameTruncated, got %v", err)
	}
}

func TestFrameReaderTruncatedPayload(t *testing.T) {
	buf := new(bytes.Buffer)

	// Write length prefix for 100 bytes
	var lengthBuf [LengthPrefixSize]byte
	binary.BigEndian.PutUint32(lengthBuf[:], 100)
	buf.Write(lengthBuf[:])

	// Write only 50 bytes of payload
	buf.Write(bytes.Repeat([]byte("x"), 50))

	reader := NewFrameReader(buf)
	_, err := reader.ReadFrame()
	if !errors.Is(err, ErrFrameTruncated) {
		t.Errorf("expected ErrFrameTruncated, got %v", err)
	}
}

func TestFrameReaderEOF(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewFrameReader(buf)

	_, err := reader.ReadFrame()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestFramerBidirectional(t *testing.T) {
	// Simulate a bidirectional connection using a pipe
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	done := make(chan struct{})
	payload := []byte("test message")

	// Writer goroutine
	go func() {
		defer close(done)
		framer := NewFramer(&readWriter{r: r, w: w})
		if err := framer.WriteFrame(payload); err != nil {
			t.Errorf("WriteFrame failed: %v", err)
		}
	}()

	// Reader
	framer := NewFramer(&readWriter{r: r, w: w})
	got, err := framer.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}

	<-done
}

// readWriter combines a reader and writer for testing.
type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw *readWriter) Read(p []byte) (n int, err error) {
	return rw.r.Read(p)
}

func (rw *readWriter) Write(p []byte) (n int, err error) {
	return rw.w.Write(p)
}

func TestMultipleFrames(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewFrameWriter(buf)

	messages := [][]byte{
		[]byte("first"),
		[]byte("second"),
		[]byte("third"),
	}

	// Write all messages
	for _, msg := range messages {
		if err := writer.WriteFrame(msg); err != nil {
			t.Fatalf("WriteFrame failed: %v", err)
		}
	}

	// Read all messages
	reader := NewFrameReader(buf)
	for i, want := range messages {
		got, err := reader.ReadFrame()
		if err != nil {
			t.Fatalf("ReadFrame %d failed: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("message %d mismatch: got %q, want %q", i, got, want)
		}
	}

	// Should get EOF after all messages
	_, err := reader.ReadFrame()
	if err != io.EOF {
		t.Errorf("expected EOF after all messages, got %v", err)
	}
}

func TestFrameSize(t *testing.T) {
	if got := FrameSize(100); got != 104 {
		t.Errorf("FrameSize(100) = %d, want 104", got)
	}
	if got := FrameSize(0); got != 4 {
		t.Errorf("FrameSize(0) = %d, want 4", got)
	}
}

func BenchmarkFrameWrite(b *testing.B) {
	buf := new(bytes.Buffer)
	writer := NewFrameWriter(buf)
	payload := bytes.Repeat([]byte("x"), 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.WriteFrame(payload)
	}
}

func BenchmarkFrameRead(b *testing.B) {
	// Prepare a buffer with many frames
	buf := new(bytes.Buffer)
	writer := NewFrameWriter(buf)
	payload := bytes.Repeat([]byte("x"), 1000)

	for i := 0; i < 1000; i++ {
		writer.WriteFrame(payload)
	}

	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := NewFrameReader(bytes.NewReader(data))
		for {
			_, err := reader.ReadFrame()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
