package service

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// sendPASERequest writes a dummy PASERequest to the connection using
// length-prefixed CBOR (the commissioning wire format).
func sendPASERequest(t *testing.T, conn net.Conn) {
	t.Helper()

	req := &commissioning.PASERequest{
		MsgType:        commissioning.MsgPASERequest,
		PublicValue:    make([]byte, 65), // dummy
		ClientIdentity: []byte("test-controller"),
	}
	data, err := cbor.Marshal(req)
	if err != nil {
		t.Fatalf("marshal PASERequest: %v", err)
	}

	// Write length prefix (4 bytes, big-endian)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	if _, err := conn.Write(length); err != nil {
		t.Fatalf("write length: %v", err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write data: %v", err)
	}
}

// readCommissioningError reads a length-prefixed CBOR CommissioningError from
// the connection. Returns nil if the connection closes without sending one.
func readCommissioningError(t *testing.T, conn net.Conn) *commissioning.CommissioningError {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read 4-byte length prefix
	length := make([]byte, 4)
	if _, err := io.ReadFull(conn, length); err != nil {
		t.Fatalf("read length: %v", err)
	}

	msgLen := binary.BigEndian.Uint32(length)
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, data); err != nil {
		t.Fatalf("read data: %v", err)
	}

	msg, err := commissioning.DecodePASEMessage(data)
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}

	errMsg, ok := msg.(*commissioning.CommissioningError)
	if !ok {
		t.Fatalf("expected *CommissioningError, got %T", msg)
	}
	return errMsg
}

// dialCommissioning opens a TLS connection to the device in commissioning mode.
func dialCommissioning(t *testing.T, addr net.Addr) *tls.Conn {
	t.Helper()

	tlsConfig := transport.NewCommissioningTLSConfig()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("dialCommissioning: %v", err)
	}
	return conn
}

// TestBusyResponse_CommissioningInProgress verifies that when a commissioning
// handshake is already in progress, a second controller gets a busy response
// with RetryAfter > 0.
func TestBusyResponse_CommissioningInProgress(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.TLSAddr()

	// First controller: TLS connect + send PASERequest to hold the commissioning lock.
	conn1 := dialCommissioning(t, addr)
	defer conn1.Close()
	sendPASERequest(t, conn1)

	// Wait for server to acquire the commissioning lock.
	time.Sleep(100 * time.Millisecond)

	// Second controller: TLS connect + send PASERequest -> should get busy response.
	conn2 := dialCommissioning(t, addr)
	defer conn2.Close()
	sendPASERequest(t, conn2)

	errMsg := readCommissioningError(t, conn2)

	if errMsg.ErrorCode != commissioning.ErrCodeBusy {
		t.Errorf("ErrorCode: expected %d (ErrCodeBusy), got %d", commissioning.ErrCodeBusy, errMsg.ErrorCode)
	}
	if errMsg.RetryAfter == 0 {
		t.Error("RetryAfter: expected > 0 when commissioning in progress")
	}
}

// TestBusyResponse_CooldownActive verifies that during connection cooldown,
// a commissioning attempt gets a busy response with RetryAfter ~ remaining cooldown.
func TestBusyResponse_CooldownActive(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.TLSAddr()

	// Override cooldown to 2s for testing (non-test-mode, so cooldown is active).
	svc.config.ConnectionCooldown = 2 * time.Second
	svc.config.TestMode = false

	// First controller: TLS connect + send PASERequest to trigger cooldown.
	// The PASE won't succeed (wrong public key) but cooldown starts when
	// acceptCommissioningConnection is called.
	conn1 := dialCommissioning(t, addr)
	sendPASERequest(t, conn1)
	// Wait for the server to process and release the commissioning lock
	// (PASE will fail, releasing the lock and setting lastCommissioningAttempt).
	time.Sleep(200 * time.Millisecond)
	conn1.Close()

	// Wait a moment for the connection to fully close and lock to be released.
	time.Sleep(100 * time.Millisecond)

	// Second controller immediately: should get cooldown busy response.
	conn2 := dialCommissioning(t, addr)
	defer conn2.Close()
	sendPASERequest(t, conn2)

	errMsg := readCommissioningError(t, conn2)

	if errMsg.ErrorCode != commissioning.ErrCodeBusy {
		t.Errorf("ErrorCode: expected %d (ErrCodeBusy), got %d", commissioning.ErrCodeBusy, errMsg.ErrorCode)
	}
	if errMsg.RetryAfter == 0 {
		t.Error("RetryAfter: expected > 0 during cooldown")
	}
}

// TestBusyResponse_ZonesFullNotInCommissioningMode verifies that when all zone
// slots are occupied AND the device has exited commissioning mode, a new
// commissioning attempt still gets a proper CommissioningError with ErrCodeBusy
// (rather than the connection being silently closed by handleOperationalConnection).
func TestBusyResponse_ZonesFullNotInCommissioningMode(t *testing.T) {
	svc := startCappedDevice(t, 1)
	addr := svc.TLSAddr()

	// Fill the single zone slot.
	svc.HandleZoneConnect("zone-full-test", 2) // ZoneTypeLocal = 2

	// Exit commissioning mode -- the device is now fully operational.
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode: %v", err)
	}

	// New controller connects without a cert -> should be routed to
	// handleCommissioningConnection which will send ErrCodeBusy.
	conn := dialCommissioning(t, addr)
	defer conn.Close()
	sendPASERequest(t, conn)

	errMsg := readCommissioningError(t, conn)

	if errMsg.ErrorCode != commissioning.ErrCodeBusy {
		t.Errorf("ErrorCode: expected %d (ErrCodeBusy), got %d", commissioning.ErrCodeBusy, errMsg.ErrorCode)
	}
	if errMsg.RetryAfter != 0 {
		t.Errorf("RetryAfter: expected 0 when zones full, got %d", errMsg.RetryAfter)
	}
}

// TestBusyResponse_ZonesFull verifies that when all zone slots are occupied,
// a commissioning attempt gets a busy response with RetryAfter == 0.
func TestBusyResponse_ZonesFull(t *testing.T) {
	svc := startCappedDevice(t, 1)
	addr := svc.TLSAddr()

	// Simulate a commissioned zone to fill the slot.
	svc.HandleZoneConnect("zone-full-test", 2) // ZoneTypeLocal = 2

	// Retry: connection + PASERequest -> should get busy with RetryAfter==0.
	conn := dialCommissioning(t, addr)
	defer conn.Close()
	sendPASERequest(t, conn)

	errMsg := readCommissioningError(t, conn)

	if errMsg.ErrorCode != commissioning.ErrCodeBusy {
		t.Errorf("ErrorCode: expected %d (ErrCodeBusy), got %d", commissioning.ErrCodeBusy, errMsg.ErrorCode)
	}
	if errMsg.RetryAfter != 0 {
		t.Errorf("RetryAfter: expected 0 when zones full, got %d", errMsg.RetryAfter)
	}
}
