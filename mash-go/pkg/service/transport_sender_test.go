package service

import (
	"errors"
	"sync"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/transport"
)

// mockConnection is a minimal mock for testing TransportRequestSender.
type mockConnection struct {
	mu       sync.Mutex
	sent     [][]byte
	sendErr  error
	state    transport.ConnectionState
	onSend   func([]byte) error // Optional callback
}

func newMockConnection() *mockConnection {
	return &mockConnection{
		state: transport.StateConnected,
	}
}

func (m *mockConnection) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onSend != nil {
		if err := m.onSend(data); err != nil {
			return err
		}
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, data)
	return nil
}

func (m *mockConnection) State() transport.ConnectionState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *mockConnection) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func (m *mockConnection) SetState(state transport.ConnectionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

func (m *mockConnection) SentMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]byte, len(m.sent))
	copy(result, m.sent)
	return result
}

func TestTransportRequestSender_Send(t *testing.T) {
	conn := newMockConnection()
	sender := NewTransportRequestSender(conn)

	testData := []byte("test message")

	err := sender.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	sent := conn.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(sent))
	}

	if string(sent[0]) != string(testData) {
		t.Errorf("Sent data mismatch: got %q, want %q", sent[0], testData)
	}
}

func TestTransportRequestSender_SendError(t *testing.T) {
	conn := newMockConnection()
	conn.SetSendError(errors.New("connection closed"))

	sender := NewTransportRequestSender(conn)

	err := sender.Send([]byte("test"))
	if err == nil {
		t.Fatal("Expected error from Send")
	}

	if err.Error() != "connection closed" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestTransportRequestSender_SendMultiple(t *testing.T) {
	conn := newMockConnection()
	sender := NewTransportRequestSender(conn)

	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("message 3"),
	}

	for _, msg := range messages {
		if err := sender.Send(msg); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	sent := conn.SentMessages()
	if len(sent) != len(messages) {
		t.Fatalf("Expected %d messages sent, got %d", len(messages), len(sent))
	}

	for i, msg := range messages {
		if string(sent[i]) != string(msg) {
			t.Errorf("Message %d mismatch: got %q, want %q", i, sent[i], msg)
		}
	}
}

func TestTransportRequestSender_ImplementsInterface(t *testing.T) {
	// This test verifies that TransportRequestSender implements
	// the interaction.RequestSender interface at compile time.
	conn := newMockConnection()
	sender := NewTransportRequestSender(conn)

	// The interaction.RequestSender interface requires:
	// Send(data []byte) error
	var _ interface {
		Send([]byte) error
	} = sender
}
