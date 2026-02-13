package runner

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// ensureCommissioned checks if already commissioned; if not, connects and performs PASE.
func (r *Runner) ensureCommissioned(ctx context.Context, state *engine.ExecutionState) error {
	// Track whether we already had a live connection before ensureConnected.
	wasConnected := r.pool.Main() != nil && r.pool.Main().isConnected()

	r.debugf("ensureCommissioned: wasConnected=%v paseState=%v paseCompleted=%v",
		wasConnected,
		r.paseState != nil,
		r.paseState != nil && r.paseState.completed)

	// First ensure we're connected.
	if err := r.ensureConnected(ctx, state); err != nil {
		r.debugf("ensureCommissioned: ensureConnected failed: %v", err)
		return err
	}

	// If already commissioned AND the connection was already live (not
	// freshly established), we can reuse the existing session. If the
	// connection was dead and ensureConnected created a new one, the
	// old PASE session is invalid -- the device expects PASE on the
	// new connection, so we must redo commissioning.
	if r.paseState != nil && r.paseState.completed {
		if wasConnected {
			r.debugf("ensureCommissioned: reusing existing PASE session")
			// Always restore suite zone crypto when reusing a session.
			// Precondition handlers may have replaced it (device_has_grid_zone
			// generates a new Zone CA), and lower-level tests may have cleared
			// it (non-commissioned tests nil out zoneCA/zoneCAPool). The suite
			// zone crypto is the only crypto that matches the device's actual
			// TLS config for the reused session.
			crypto := r.suite.Crypto()
			if crypto.ZoneCAPool != nil {
				r.zoneCA = crypto.ZoneCA
				r.controllerCert = crypto.ControllerCert
				r.issuedDeviceCert = crypto.IssuedDeviceCert
				// Ensure suite zone CA is in the pool without replacing it.
				// The accumulated pool may contain CAs from other zones the
				// device still knows about. Replacing it with the suite pool
				// loses those CAs (the device may present a cert from any zone).
				if r.zoneCAPool == nil {
					r.zoneCAPool = x509.NewCertPool()
				}
				if crypto.ZoneCA != nil && crypto.ZoneCA.Certificate != nil {
					r.zoneCAPool.AddCert(crypto.ZoneCA.Certificate)
				}
				r.debugf("ensureCommissioned: restored suite zone crypto")
			}
			state.Set(KeySessionEstablished, true)
			state.Set(KeyConnectionEstablished, true)
			if r.paseState.sessionKey != nil {
				state.Set(StateSessionKey, r.paseState.sessionKey)
				state.Set(StateSessionKeyLen, len(r.paseState.sessionKey))
			}
			return nil
		}
		// Connection was re-established -- old PASE session is stale.
		r.debugf("ensureCommissioned: connection was re-established, clearing stale PASE state")
		r.paseState = nil
	}

	// Create a synthetic step to drive handleCommission.
	// _from_precondition tells handleCommission to skip creating a tracking
	// connection -- ensureCommissioned calls transitionToOperational after,
	// which handles the operational connection and zone registration.
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{
			ParamFromPrecondition: true,
		},
	}

	// Pass setup_code from config if available.
	if r.config.SetupCode != "" {
		step.Params["setup_code"] = r.config.SetupCode
	}

	_, err := r.handleCommission(ctx, step, state)
	if err != nil {
		classified := classifyPASEError(err)

		// DEC-065: If the device rejects with a cooldown error, wait for the
		// remaining cooldown and retry once. This handles transitions between
		// auto-PICS discovery and test execution where the cooldown from the
		// previous commissioning is still active.
		if wait := cooldownRemaining(err); wait > 0 {
			r.debugf("ensureCommissioned: cooldown active, waiting %s", wait.Round(time.Millisecond))
			if err := contextSleep(ctx, wait); err != nil {
				return fmt.Errorf("cooldown wait cancelled: %w", err)
			}
			// Clear connection but preserve suite zone state so multi-zone
			// precondition loops don't lose the suite zone on retry.
			r.disconnectConnection()
			r.zoneCA = nil
			r.controllerCert = nil
			r.zoneCAPool = nil
			r.issuedDeviceCert = nil
			if connErr := r.ensureConnected(ctx, state); connErr != nil {
				return fmt.Errorf("precondition commission cooldown retry connect failed: %w", connErr)
			}
			_, err = r.handleCommission(ctx, step, state)
			if err != nil {
				return fmt.Errorf("precondition commission failed after cooldown wait: %w", err)
			}
			if r.paseState == nil || !r.paseState.completed {
				return fmt.Errorf("precondition commission: PASE did not complete after cooldown retry")
			}
			if err := r.transitionToOperational(state); err != nil {
				return err
			}
			if r.isSuiteZoneCommission() {
				r.suite.Record(r.suite.ZoneID(), CryptoState{
					ZoneCA:           r.zoneCA,
					ControllerCert:   r.controllerCert,
					ZoneCAPool:       r.zoneCAPool,
					IssuedDeviceCert: r.issuedDeviceCert,
				})
				r.debugf("ensureCommissioned: updated suite zone crypto after cooldown retry commission")
			}
			return nil
		}

		// Device errors (wrong setup code, zone type exists) are not retryable.
		if Category(classified) == ErrCatDevice {
			return fmt.Errorf("precondition commission rejected by device: %w", err)
		}

		// Infrastructure errors (network, zone slots full via auto-evict): retry
		// after disconnect + delay. The device may still be cycling its
		// commissioning window in test mode, especially after zone removals.
		const maxRetries = 2
		if Category(classified) == ErrCatInfrastructure {
			for retry := 1; retry <= maxRetries; retry++ {
				// Clear connection but preserve suite zone state.
				r.disconnectConnection()
				r.zoneCA = nil
				r.controllerCert = nil
				r.zoneCAPool = nil
				r.issuedDeviceCert = nil
				if err := contextSleep(ctx, 1*time.Second); err != nil {
					return fmt.Errorf("infrastructure retry wait cancelled: %w", err)
				}
				if connErr := r.ensureConnected(ctx, state); connErr != nil {
					return fmt.Errorf("precondition commission retry %d connect failed: %w", retry, connErr)
				}
				_, err = r.handleCommission(ctx, step, state)
				if err == nil {
					if r.paseState == nil || !r.paseState.completed {
						return fmt.Errorf("precondition commission: PASE did not complete on retry %d", retry)
					}
					if trErr := r.transitionToOperational(state); trErr != nil {
						return trErr
					}
					if r.isSuiteZoneCommission() {
						r.suite.Record(r.suite.ZoneID(), CryptoState{
							ZoneCA:           r.zoneCA,
							ControllerCert:   r.controllerCert,
							ZoneCAPool:       r.zoneCAPool,
							IssuedDeviceCert: r.issuedDeviceCert,
						})
						r.debugf("ensureCommissioned: updated suite zone crypto after infra retry commission")
					}
					return nil
				}
				retryClassified := classifyPASEError(err)
				if Category(retryClassified) != ErrCatInfrastructure {
					break
				}
			}
			return fmt.Errorf("precondition commission failed after %d retries: %w", maxRetries, err)
		}

		// Protocol errors: fail immediately.
		return fmt.Errorf("precondition commission failed: %w", err)
	}

	// handleCommission may return nil error for PASE protocol failures
	// (device-sent error codes). Check paseState to detect these.
	if r.paseState == nil || !r.paseState.completed {
		return fmt.Errorf("precondition commission: PASE handshake did not complete")
	}

	if err := r.transitionToOperational(state); err != nil {
		return err
	}

	// After a fresh commission that re-creates the suite zone, update the
	// saved suite crypto so subsequent session-reuse restores use the correct
	// (current) crypto. Only update when this commission IS the suite zone;
	// secondary zones (GRID/LOCAL from two_zones_connected) must not
	// overwrite suite crypto.
	if r.isSuiteZoneCommission() {
		r.suite.Record(r.suite.ZoneID(), CryptoState{
			ZoneCA:           r.zoneCA,
			ControllerCert:   r.controllerCert,
			ZoneCAPool:       r.zoneCAPool,
			IssuedDeviceCert: r.issuedDeviceCert,
		})
		r.debugf("ensureCommissioned: updated suite zone crypto after fresh commission")
	}

	return nil
}

// isSuiteZoneCommission returns true when the most recent commission created
// (or re-created) the suite zone. Secondary zones such as GRID/LOCAL from
// two_zones_connected must not overwrite the suite zone's saved crypto.
//
// Heuristic: if the suite zone connection is still alive, this commission
// created a secondary zone alongside it. If the suite zone connection is
// dead or gone, this commission replaces it.
func (r *Runner) isSuiteZoneCommission() bool {
	if r.suite.ZoneID() == "" {
		return false
	}
	if r.suite.Conn() != nil && r.suite.Conn().isConnected() {
		return false
	}
	return true
}

// transitionToOperational closes the commissioning connection and establishes
// a new operational TLS connection using the controller certificate received
// during cert exchange. This implements DEC-066: the device closes the
// commissioning connection after sending CertInstallAck, and the controller
// must reconnect with operational certificates.
//
// The new connection is registered in activeZoneConns so that
// closeActiveZoneConns can send RemoveZone and close it between tests.
// Without this registration, the zone leaks on the device and subsequent
// tests fail with "zone slots full".
func (r *Runner) transitionToOperational(state *engine.ExecutionState) error {
	if r.paseState == nil || r.paseState.sessionKey == nil {
		return fmt.Errorf("no PASE session to transition")
	}

	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// DEC-066: Close the commissioning connection.
	// The device has already closed its end after sending CertInstallAck.
	if r.pool.Main() != nil {
		r.debugf("transitionToOperational: closing commissioning connection")
		_ = r.pool.Main().Close()
		r.pool.SetMain(nil)
	}

	// DEC-066: Establish new operational TLS connection.
	// Retry the dial briefly in case the device hasn't finished registering
	// the zone as awaiting reconnection.
	r.debugf("transitionToOperational: reconnecting with operational TLS")

	target := r.getTarget(nil)
	ctx := context.Background()
	crypto := r.WorkingCrypto()
	tlsConn, dialErr := dialWithRetry(ctx, 3, func() (*tls.Conn, error) {
		return r.dialer.DialOperational(ctx, target, crypto)
	})
	if dialErr != nil {
		return fmt.Errorf("operational reconnection failed: %w", dialErr)
	}

	// Create new connection wrapper
	newConn := &Connection{
		tlsConn: tlsConn,
		framer:  transport.NewFramer(tlsConn),
		state:   ConnOperational,
	}
	r.pool.SetMain(newConn)
	state.Set(StateConnection, newConn)
	// Record timestamp for verify_timing (TC-TRANS-004).
	state.Set(StateOperationalConnEstablished, time.Now())

	// Verify the device is processing protocol messages on this connection.
	if err := r.waitForOperationalReady(2 * time.Second); err != nil {
		r.debugf("transitionToOperational: %v (continuing)", err)
	}

	// Register the commissioned zone so closeActiveZoneConns can clean it up.
	connKey := "main-" + zoneID
	r.pool.TrackZone(connKey, newConn, zoneID)
	r.debugf("transitionToOperational: reconnected and registered zone %s in activeZoneConns", zoneID)

	return nil
}

// reconnectToZone re-establishes an operational TLS connection to an existing
// suite zone using stored crypto material. No PASE/cert exchange needed.
// Returns an error if the reconnection fails.
func (r *Runner) reconnectToZone(state *engine.ExecutionState) error {
	if r.suite.ZoneID() == "" {
		return fmt.Errorf("no suite zone to reconnect to")
	}
	if r.zoneCAPool == nil || r.controllerCert == nil {
		// Try restoring from saved suite zone crypto.
		crypto := r.suite.Crypto()
		if crypto.ZoneCAPool != nil && crypto.ControllerCert != nil {
			r.zoneCA = crypto.ZoneCA
			r.controllerCert = crypto.ControllerCert
			r.zoneCAPool = crypto.ZoneCAPool
			r.issuedDeviceCert = crypto.IssuedDeviceCert
			r.debugf("reconnectToZone: restored suite zone crypto")
		} else {
			return fmt.Errorf("no crypto material for reconnection")
		}
	}

	r.debugf("reconnectToZone: reconnecting to zone %s", r.suite.ZoneID())

	target := r.getTarget(nil)
	ctx := context.Background()
	crypto := r.WorkingCrypto()
	tlsConn, dialErr := dialWithRetry(ctx, 3, func() (*tls.Conn, error) {
		return r.dialer.DialOperational(ctx, target, crypto)
	})
	if dialErr != nil {
		return fmt.Errorf("reconnectToZone failed: %w", dialErr)
	}

	newConn := &Connection{
		tlsConn: tlsConn,
		framer:  transport.NewFramer(tlsConn),
		state:   ConnOperational,
	}
	r.pool.SetMain(newConn)
	state.Set(StateConnection, newConn)

	// Verify the device accepts us on this connection.
	if err := r.waitForOperationalReady(2 * time.Second); err != nil {
		r.debugf("reconnectToZone: readiness check failed: %v", err)
		r.pool.Main().transitionTo(ConnDisconnected)
		return fmt.Errorf("reconnectToZone readiness failed: %w", err)
	}

	// Store on suite session (not in pool -- suite zone lives outside pool).
	r.suite.SetConn(newConn)
	r.debugf("reconnectToZone: reconnected to zone %s", r.suite.ZoneID())

	return nil
}

// cooldownRemaining extracts the remaining cooldown duration from a PASE
// handshake error. Returns 0 if the error is not a cooldown rejection.
// Error format: "...cooldown active (123.456ms remaining)..."
func cooldownRemaining(err error) time.Duration {
	if err == nil {
		return 0
	}
	msg := err.Error()
	const marker = "cooldown active ("
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0
	}
	rest := msg[idx+len(marker):]
	endIdx := strings.Index(rest, " remaining)")
	if endIdx < 0 {
		return 0
	}
	d, parseErr := time.ParseDuration(rest[:endIdx])
	if parseErr != nil {
		return 0
	}
	// Add a buffer that covers TLS reconnection overhead (~35ms) plus margin.
	return d + 200*time.Millisecond
}

// isTransientError returns true for errors that may resolve on retry.
// Checks for classified infrastructure errors first, then falls back to
// IO-level pattern matching for unclassified errors.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	// Classified errors: only infrastructure is retryable.
	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Category == ErrCatInfrastructure
	}
	// Unclassified: fall back to IO pattern matching.
	return isIOError(err)
}

// deriveZoneIDFromSecret derives a zone ID from a PASE shared secret
// using the same SHA-256 derivation as the device side.
func deriveZoneIDFromSecret(secret []byte) string {
	hash := sha256.Sum256(secret)
	return hex.EncodeToString(hash[:8])
}

// commissionSuiteZone performs one-time suite-level commissioning.
// The suite zone is commissioned as ZoneTypeTest so it doesn't count
// against the device's MaxZones and is transparent to device logic.
func (r *Runner) commissionSuiteZone(ctx context.Context) error {
	r.debugf("commissionSuiteZone: commissioning device for suite")
	state := engine.NewExecutionState(ctx)

	r.commissionZoneType = cert.ZoneTypeTest
	if err := r.ensureCommissioned(ctx, state); err != nil {
		r.commissionZoneType = 0
		return fmt.Errorf("suite commissioning: %w", err)
	}
	r.commissionZoneType = 0

	r.recordSuiteZone()
	r.debugf("commissionSuiteZone: suite zone established (id=%s key=%s)", r.suite.ZoneID(), r.suite.ConnKey())
	return nil
}

// recordSuiteZone captures the current commissioned zone as the suite zone.
// It also saves the crypto state so it can be restored after lower-level
// tests clear the working crypto (non-commissioned tests nil out zoneCA
// and zoneCAPool to avoid stale TLS configs).
//
// The suite zone connection is moved out of the ConnPool so that pool-level
// operations (close, scan, cleanup) never touch it.
func (r *Runner) recordSuiteZone() {
	if r.paseState == nil || r.paseState.sessionKey == nil {
		return
	}
	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)
	r.suite.Record(zoneID, CryptoState{
		ZoneCA:           r.zoneCA,
		ControllerCert:   r.controllerCert,
		ZoneCAPool:       r.zoneCAPool,
		IssuedDeviceCert: r.issuedDeviceCert,
	})

	// Move the suite zone connection from pool to suite session.
	connKey := r.suite.ConnKey()
	if conn := r.pool.Zone(connKey); conn != nil {
		r.suite.SetConn(conn)
		r.pool.UntrackZone(connKey)
	} else if r.pool.Main() != nil && r.pool.Main().isConnected() {
		r.suite.SetConn(r.pool.Main())
	}
}

// removeSuiteZone sends RemoveZone for the suite zone and clears all state.
func (r *Runner) removeSuiteZone() {
	r.debugf("removeSuiteZone: tearing down suite zone %s", r.suite.ZoneID())
	r.sendRemoveZone()
	r.closeAllZoneConns()
	r.ensureDisconnected()
}
