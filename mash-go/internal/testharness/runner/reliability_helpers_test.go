package runner

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestDialWithTransientRetry_RetriesTransientThenSucceeds(t *testing.T) {
	attempts := 0
	conn, err := dialWithTransientRetry(context.Background(), 3, func() (*tls.Conn, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("read tcp 127.0.0.1:1->127.0.0.1:2: read: connection reset by peer")
		}
		return &tls.Conn{}, nil
	})
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDialWithTransientRetry_DoesNotRetryProtocolError(t *testing.T) {
	attempts := 0
	_, err := dialWithTransientRetry(context.Background(), 3, func() (*tls.Conn, error) {
		attempts++
		return nil, errors.New("tls: bad certificate")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for non-transient error, got %d", attempts)
	}
}

func TestDetectPostHandshakeClose_WaitsLongEnoughForDelayedClose(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		time.Sleep(250 * time.Millisecond)
		_ = c2.Close()
	}()

	closed, err := detectPostHandshakeClose(c1, 500*time.Millisecond)
	if !closed {
		t.Fatal("expected closed=true")
	}
	if err == nil || !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF close error, got: %v", err)
	}
}

func TestHandleConnect_DelayedInvalidCertRejectionDetected(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	zoneCA := newTestZoneCA(t)
	r.connMgr.SetZoneCA(zoneCA)
	r.connMgr.SetZoneCAPool(zoneCA.TLSClientCAs())

	// TLS server accepts handshake, then closes after a delay to emulate
	// post-handshake cert rejection timing.
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{selfSignedCert(t)},
		MinVersion:   tls.VersionTLS13,
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		if tlsConn, ok := conn.(*tls.Conn); ok {
			_ = tlsConn.Handshake()
		}
		time.Sleep(250 * time.Millisecond)
		_ = conn.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"target":        listener.Addr().String(),
		"commissioning": true,
		"cert_chain": []any{
			CertTypeInvalidSignature,
		},
		"alpn": "mash/1",
	}}

	out, err := r.handleConnect(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyConnected] != false {
		t.Fatalf("expected connected=false, got %v", out[KeyConnected])
	}
	if out[KeyTLSAlert] != "bad_certificate" {
		t.Fatalf("expected tls_alert=bad_certificate, got %v", out[KeyTLSAlert])
	}
	<-done
}
