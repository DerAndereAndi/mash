package service

// Sendable is the interface for a connection that can send data.
// This is implemented by transport.Connection.
type Sendable interface {
	Send(data []byte) error
}

// TransportRequestSender adapts a transport connection to the
// interaction.RequestSender interface.
//
// This adapter enables the interaction.Client to send requests
// over a transport.Connection after PASE commissioning is complete.
type TransportRequestSender struct {
	conn Sendable
}

// NewTransportRequestSender creates a new sender that wraps a connection.
func NewTransportRequestSender(conn Sendable) *TransportRequestSender {
	return &TransportRequestSender{conn: conn}
}

// Send sends raw data over the underlying transport connection.
// This implements the interaction.RequestSender interface.
func (s *TransportRequestSender) Send(data []byte) error {
	return s.conn.Send(data)
}
