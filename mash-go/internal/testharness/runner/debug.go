package runner

import (
	"fmt"
	stdlog "log"
	"strings"
	"time"
)

// RunnerSnapshot captures the runner's inter-test state for debugging and
// assertions. It records the state of connections, zones, PASE, and
// certificates at a point in time.
type RunnerSnapshot struct {
	// Timestamp is when the snapshot was taken.
	Timestamp time.Time

	// MainConn describes the primary connection.
	MainConn ConnSnapshot

	// PASECompleted indicates whether the PASE session is established.
	PASECompleted bool
	// HasSessionKey indicates whether a session key exists.
	HasSessionKey bool

	// HasZoneCA indicates whether a Zone CA is present.
	HasZoneCA bool
	// HasControllerCert indicates whether a controller cert is present.
	HasControllerCert bool
	// HasZoneCAPool indicates whether a Zone CA pool is present.
	HasZoneCAPool bool

	// ActiveZones lists the names of active zone connections and their state.
	ActiveZones map[string]ConnSnapshot

	// ActiveZoneIDs lists the zone name -> derived zone ID mapping.
	ActiveZoneIDs map[string]string

	// LastDeviceConnClose is when zone connections were last closed.
	LastDeviceConnClose time.Time

	// CommissionZoneType is the current zone type override.
	CommissionZoneType int
}

// ConnSnapshot captures the state of a single connection.
type ConnSnapshot struct {
	Connected    bool
	HasTLSConn   bool
	HasRawConn   bool
	HasFramer    bool
	PendingNotif int
}

// snapshot returns a point-in-time snapshot of the runner's inter-test state.
func (r *Runner) snapshot() RunnerSnapshot {
	s := RunnerSnapshot{
		Timestamp:           time.Now(),
		HasZoneCA:           r.connMgr.ZoneCA() != nil,
		HasControllerCert:   r.connMgr.ControllerCert() != nil,
		HasZoneCAPool:       r.connMgr.ZoneCAPool() != nil,
		ActiveZones:         make(map[string]ConnSnapshot),
		ActiveZoneIDs:       make(map[string]string),
		LastDeviceConnClose: r.connMgr.LastDeviceConnClose(),
		CommissionZoneType:  int(r.connMgr.CommissionZoneType()),
	}

	if r.pool.Main() != nil {
		s.MainConn = connSnapshot(r.pool.Main())
	}

	if ps := r.connMgr.PASEState(); ps != nil {
		s.PASECompleted = ps.completed
		s.HasSessionKey = ps.sessionKey != nil
	}

	for _, key := range r.pool.ZoneKeys() {
		if conn := r.pool.Zone(key); conn != nil {
			s.ActiveZones[key] = connSnapshot(conn)
		}
	}
	for _, key := range r.pool.ZoneKeys() {
		s.ActiveZoneIDs[key] = r.pool.ZoneID(key)
	}

	return s
}

func connSnapshot(c *Connection) ConnSnapshot {
	return ConnSnapshot{
		Connected:    c.isConnected(),
		HasTLSConn:   c.tlsConn != nil,
		HasRawConn:   c.conn != nil,
		HasFramer:    c.framer != nil,
		PendingNotif: len(c.pendingNotifications),
	}
}

// String returns a human-readable summary of the snapshot for debug logging.
func (s RunnerSnapshot) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "conn={connected:%v tls:%v raw:%v framer:%v}",
		s.MainConn.Connected, s.MainConn.HasTLSConn, s.MainConn.HasRawConn, s.MainConn.HasFramer)
	fmt.Fprintf(&b, " pase={completed:%v key:%v}", s.PASECompleted, s.HasSessionKey)
	fmt.Fprintf(&b, " certs={zoneCA:%v ctrl:%v pool:%v}",
		s.HasZoneCA, s.HasControllerCert, s.HasZoneCAPool)
	if len(s.ActiveZones) > 0 {
		fmt.Fprintf(&b, " zones={")
		first := true
		for name, zc := range s.ActiveZones {
			if !first {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:{connected:%v tls:%v raw:%v}", name, zc.Connected, zc.HasTLSConn, zc.HasRawConn)
			first = false
		}
		b.WriteString("}")
	} else {
		b.WriteString(" zones={}")
	}
	if !s.LastDeviceConnClose.IsZero() {
		fmt.Fprintf(&b, " lastClose=%s ago", time.Since(s.LastDeviceConnClose).Round(time.Millisecond))
	}
	return b.String()
}

// HasPhantomSocket returns true if the main connection has an open socket
// despite being marked as disconnected.
func (s RunnerSnapshot) HasPhantomSocket() bool {
	return !s.MainConn.Connected && (s.MainConn.HasTLSConn || s.MainConn.HasRawConn)
}

// HasPhantomZoneSocket returns true if any zone connection has an open socket
// despite being marked as disconnected.
func (s RunnerSnapshot) HasPhantomZoneSocket() (string, bool) {
	for name, zc := range s.ActiveZones {
		if !zc.Connected && (zc.HasTLSConn || zc.HasRawConn) {
			return name, true
		}
	}
	return "", false
}

// debugf logs a debug message when the runner's Debug config is enabled.
func (r *Runner) debugf(format string, args ...any) {
	if r.config == nil || !r.config.Debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	stdlog.Printf("[DEBUG] %s", msg)
}

// debugSnapshot logs the current runner state when debug is enabled.
func (r *Runner) debugSnapshot(label string) {
	if r.config == nil || !r.config.Debug {
		return
	}
	s := r.snapshot()
	stdlog.Printf("[DEBUG] %s: %s", label, s.String())
}
