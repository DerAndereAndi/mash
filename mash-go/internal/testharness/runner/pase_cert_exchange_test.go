package runner

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// pipeReadWriter combines a PipeReader and PipeWriter into an io.ReadWriter.
type pipeReadWriter struct {
	io.Reader
	io.Writer
}

// TestFramerSyncConn_Send verifies the adapter delegates to framer.WriteFrame.
func TestFramerSyncConn_Send(t *testing.T) {
	// Create a bidirectional pipe pair.
	aR, aW := io.Pipe()
	bR, bW := io.Pipe()
	defer aR.Close()
	defer aW.Close()
	defer bR.Close()
	defer bW.Close()

	// Writer side: reads from bR (unused), writes to aW.
	writerFramer := transport.NewFramer(&pipeReadWriter{Reader: bR, Writer: aW})
	sc := &framerSyncConn{framer: writerFramer}

	// Reader side: reads from aR, writes to bW (unused).
	readerFramer := transport.NewFramer(&pipeReadWriter{Reader: aR, Writer: bW})

	payload := []byte("hello")

	go func() {
		if err := sc.Send(payload); err != nil {
			t.Errorf("Send failed: %v", err)
		}
	}()

	got, err := readerFramer.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %v, want %v", got, payload)
	}
}

// TestFramerSyncConn_ReadFrame verifies the adapter delegates to framer.ReadFrame.
func TestFramerSyncConn_ReadFrame(t *testing.T) {
	aR, aW := io.Pipe()
	bR, bW := io.Pipe()
	defer aR.Close()
	defer aW.Close()
	defer bR.Close()
	defer bW.Close()

	writeFramer := transport.NewFramer(&pipeReadWriter{Reader: bR, Writer: aW})
	readFramer := transport.NewFramer(&pipeReadWriter{Reader: aR, Writer: bW})
	sc := &framerSyncConn{framer: readFramer}

	payload := []byte("world")

	go func() {
		if err := writeFramer.WriteFrame(payload); err != nil {
			t.Errorf("WriteFrame failed: %v", err)
		}
	}()

	got, err := sc.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %v, want %v", got, payload)
	}
}

// TestFramerSyncConn_ImplementsSyncConnection verifies the adapter satisfies the interface.
func TestFramerSyncConn_ImplementsSyncConnection(t *testing.T) {
	var _ service.SyncConnection = (*framerSyncConn)(nil)
}

// TestCertExchange_FullFlow simulates the 4-message cert exchange after PASE.
// It creates a mock device on the other end of a pipe that receives
// CertRenewalRequest, responds with a CSR, receives Install, and sends Ack.
func TestCertExchange_FullFlow(t *testing.T) {
	// Create two pipes: one for each direction.
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	defer clientR.Close()
	defer serverW.Close()
	defer serverR.Close()
	defer clientW.Close()

	clientFramer := transport.NewFramer(&pipeReadWriter{Reader: clientR, Writer: clientW})
	serverFramer := transport.NewFramer(&pipeReadWriter{Reader: serverR, Writer: serverW})

	// Generate a device key pair for the mock device.
	deviceKeyPair, err := cert.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Run mock device in background.
	deviceErr := make(chan error, 1)
	go func() {
		deviceErr <- mockDeviceCertExchange(serverFramer, deviceKeyPair)
	}()

	// Set up runner with a fake PASE session key.
	r := &Runner{
		config: &Config{},
		pool:   NewConnPool(func(string, ...any) {}, nil),
		paseState: &PASEState{
			sessionKey: []byte("test-session-key-for-zone-deriv"),
			completed:  true,
		},
	}
	r.pool.SetMain(&Connection{
		framer: clientFramer,
		state:  ConnTLSConnected,
	})

	// Perform cert exchange.
	ctx := context.Background()
	deviceID, err := r.performCertExchange(ctx)
	if err != nil {
		t.Fatalf("performCertExchange: %v", err)
	}

	// Wait for mock device to finish.
	if devErr := <-deviceErr; devErr != nil {
		t.Fatalf("mock device error: %v", devErr)
	}

	// Verify: zoneCA is set.
	if r.zoneCA == nil {
		t.Error("expected zoneCA to be set")
	}

	// Verify: zoneCAPool is set.
	if r.zoneCAPool == nil {
		t.Error("expected zoneCAPool to be set")
	}

	// Verify: controllerCert is set and signed by Zone CA.
	if r.controllerCert == nil {
		t.Fatal("expected controllerCert to be set")
	}
	if r.controllerCert.Certificate == nil {
		t.Fatal("expected controllerCert.Certificate to be set")
	}
	// The controller cert's issuer should match the zone CA's subject.
	if r.controllerCert.Certificate.Issuer.CommonName != r.zoneCA.Certificate.Subject.CommonName {
		t.Errorf("controller cert issuer %q != zone CA subject %q",
			r.controllerCert.Certificate.Issuer.CommonName,
			r.zoneCA.Certificate.Subject.CommonName)
	}

	// Verify: device ID was extracted.
	if deviceID == "" {
		t.Error("expected non-empty device ID")
	}

	// Verify: zone ID derivation.
	expectedZoneID := deriveZoneIDFromSecret([]byte("test-session-key-for-zone-deriv"))
	if r.zoneCA.ZoneID != expectedZoneID {
		t.Errorf("zone ID %q != expected %q", r.zoneCA.ZoneID, expectedZoneID)
	}
}

// mockDeviceCertExchange simulates the device side of the cert exchange.
// Receives CertRenewalRequest, responds with CSR, receives Install, sends Ack.
func mockDeviceCertExchange(framer *transport.Framer, keyPair *cert.KeyPair) error {
	// Step 1: Read CertRenewalRequest
	reqData, err := framer.ReadFrame()
	if err != nil {
		return err
	}

	msg, err := commissioning.DecodeRenewalMessage(reqData)
	if err != nil {
		return err
	}

	req, ok := msg.(*commissioning.CertRenewalRequest)
	if !ok {
		return err
	}
	_ = req // Used for zone CA verification in real device

	// Step 2: Generate and send CSR
	csrDER, err := cert.CreateCSR(keyPair, &cert.CSRInfo{
		Identity: cert.DeviceIdentity{DeviceID: "mock-device"},
	})
	if err != nil {
		return err
	}

	csrResp := &commissioning.CertRenewalCSR{
		MsgType: commissioning.MsgCertRenewalCSR,
		CSR:     csrDER,
	}
	csrData, err := cbor.Marshal(csrResp)
	if err != nil {
		return err
	}
	if err := framer.WriteFrame(csrData); err != nil {
		return err
	}

	// Step 3: Read CertRenewalInstall
	installData, err := framer.ReadFrame()
	if err != nil {
		return err
	}

	msg, err = commissioning.DecodeRenewalMessage(installData)
	if err != nil {
		return err
	}

	_, ok = msg.(*commissioning.CertRenewalInstall)
	if !ok {
		return err
	}

	// Step 4: Send Ack
	ack := &commissioning.CertRenewalAck{
		MsgType:        commissioning.MsgCertRenewalAck,
		Status:         commissioning.RenewalStatusSuccess,
		ActiveSequence: 2,
	}
	ackData, err := cbor.Marshal(ack)
	if err != nil {
		return err
	}
	return framer.WriteFrame(ackData)
}
