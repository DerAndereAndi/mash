package runner

import (
	"crypto/x509"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

// SuiteSession manages the suite-level zone that persists across tests.
// It is the single source of truth for zone crypto material.
type SuiteSession interface {
	// ZoneID returns the suite zone ID, or "" if not commissioned.
	ZoneID() string

	// ConnKey returns the activeZoneConns key for the suite zone (e.g. "main-<zoneID>").
	ConnKey() string

	// IsCommissioned returns true if a suite zone has been established.
	IsCommissioned() bool

	// Crypto returns the current zone crypto material.
	// Returns nil values if not commissioned.
	Crypto() CryptoState

	// Record saves the current commissioning result as the suite zone.
	// Called after successful PASE + cert exchange + operational transition.
	Record(zoneID string, crypto CryptoState)

	// Clear removes all suite zone state.
	// Called during suite teardown or fresh_commission.
	Clear()
}

// CryptoState bundles the crypto material from a commissioning session.
// This replaces the duplicated fields (zoneCA/suiteZoneCA, etc.).
type CryptoState struct {
	ZoneCA           *cert.ZoneCA
	ControllerCert   *cert.OperationalCert
	ZoneCAPool       *x509.CertPool
	IssuedDeviceCert *x509.Certificate
}

type suiteSessionImpl struct {
	zoneID  string
	connKey string
	crypto  CryptoState
}

// NewSuiteSession creates a new empty SuiteSession.
func NewSuiteSession() SuiteSession {
	return &suiteSessionImpl{}
}

func (s *suiteSessionImpl) ZoneID() string      { return s.zoneID }
func (s *suiteSessionImpl) ConnKey() string      { return s.connKey }
func (s *suiteSessionImpl) IsCommissioned() bool { return s.zoneID != "" }
func (s *suiteSessionImpl) Crypto() CryptoState  { return s.crypto }

func (s *suiteSessionImpl) Record(zoneID string, crypto CryptoState) {
	s.zoneID = zoneID
	s.connKey = "main-" + zoneID
	s.crypto = crypto
}

func (s *suiteSessionImpl) Clear() {
	s.zoneID = ""
	s.connKey = ""
	s.crypto = CryptoState{}
}
