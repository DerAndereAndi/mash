package commissioning

import (
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/hkdf"
)

// SPAKE2+ protocol constants.
const (
	// SharedSecretSize is the size of the derived shared secret in bytes.
	SharedSecretSize = 32

	// ConfirmationSize is the size of the confirmation MAC in bytes.
	ConfirmationSize = 32
)

// SPAKE2+ errors.
var (
	ErrInvalidPublicKey   = errors.New("invalid public key")
	ErrConfirmationFailed = errors.New("confirmation failed")
	ErrInvalidVerifier    = errors.New("invalid verifier")
)

// Curve parameters for P-256.
var curve = elliptic.P256()

// M and N are fixed generator points for SPAKE2+ on P-256.
// These are derived by hashing fixed strings to curve points.
// Values from RFC 9383 test vectors for P-256.
var (
	// M = HashToPoint("SPAKE2+-P256-SHA256-HKDF-SHA256-HMAC-SHA256 M")
	pointM = &curvePoint{
		x: mustHexBigInt("886e2f97ace46e55ba9dd7242579f2993b64e16ef3dcab95afd497333d8fa12f"),
		y: mustHexBigInt("5ff355163e43ce224e0b0e65ff02ac8e5c7be09419c785e0ca547d55a12e2d20"),
	}

	// N = HashToPoint("SPAKE2+-P256-SHA256-HKDF-SHA256-HMAC-SHA256 N")
	pointN = &curvePoint{
		x: mustHexBigInt("d8bbd6c639c62937b04d997f38c3770719c629d7014d49a24b4f98baa1292b49"),
		y: mustHexBigInt("07d60aa6bfade45008a636337f5168c64d9bd36034808cd564490b1e656edbe7"),
	}
)

// curvePoint represents a point on the elliptic curve.
type curvePoint struct {
	x, y *big.Int
}

// mustHexBigInt parses a hex string to big.Int or panics.
func mustHexBigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex string: " + s)
	}
	return n
}

// Verifier contains the server-side verification material.
// This is stored by the device and derived from the setup code during registration.
type Verifier struct {
	// W0 is the first verification value (scalar).
	W0 []byte

	// L is the second verification value (curve point, compressed).
	L []byte

	// Identity is the client identity used during registration.
	Identity []byte
}

// GenerateVerifier creates a verifier from a setup code.
// This is called by the device during manufacturing/registration.
func GenerateVerifier(setupCode SetupCode, clientIdentity, serverIdentity []byte) (*Verifier, error) {
	// Derive w0 and w1 from the setup code using HKDF
	password := setupCode.Bytes()

	// Create context: client_identity || server_identity
	context := append(append([]byte{}, clientIdentity...), serverIdentity...)

	// Use HKDF to derive w0 and w1
	// In practice, a proper MHF like Argon2 or scrypt should be used first
	hkdfReader := hkdf.New(sha256.New, password, context, []byte("SPAKE2+-P256-SHA256 w"))

	w0Bytes := make([]byte, 32)
	w1Bytes := make([]byte, 32)

	if _, err := io.ReadFull(hkdfReader, w0Bytes); err != nil {
		return nil, fmt.Errorf("failed to derive w0: %w", err)
	}
	if _, err := io.ReadFull(hkdfReader, w1Bytes); err != nil {
		return nil, fmt.Errorf("failed to derive w1: %w", err)
	}

	// w0 and w1 as scalars (mod n)
	w0 := new(big.Int).SetBytes(w0Bytes)
	w1 := new(big.Int).SetBytes(w1Bytes)
	w0.Mod(w0, curve.Params().N)
	w1.Mod(w1, curve.Params().N)

	// L = w1 * G
	lx, ly := curve.ScalarBaseMult(w1.Bytes())

	// Compress L point
	lBytes := elliptic.MarshalCompressed(curve, lx, ly)

	return &Verifier{
		W0:       w0.Bytes(),
		L:        lBytes,
		Identity: clientIdentity,
	}, nil
}

// SPAKE2PlusClient represents the client side of the SPAKE2+ exchange.
type SPAKE2PlusClient struct {
	setupCode      SetupCode
	clientIdentity []byte
	serverIdentity []byte

	// Ephemeral private key
	x *big.Int

	// Derived values
	w0 *big.Int
	w1 *big.Int

	// Exchange values
	pA []byte // Our public value
	pB []byte // Server's public value

	// Shared secrets
	sharedSecret []byte
	confirmKey   []byte
}

// NewSPAKE2PlusClient creates a new SPAKE2+ client.
func NewSPAKE2PlusClient(setupCode SetupCode, clientIdentity, serverIdentity []byte) (*SPAKE2PlusClient, error) {
	// Generate ephemeral private key
	x, err := rand.Int(rand.Reader, curve.Params().N)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Derive w0 and w1 from setup code
	password := setupCode.Bytes()
	context := append(append([]byte{}, clientIdentity...), serverIdentity...)
	hkdfReader := hkdf.New(sha256.New, password, context, []byte("SPAKE2+-P256-SHA256 w"))

	w0Bytes := make([]byte, 32)
	w1Bytes := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, w0Bytes); err != nil {
		return nil, fmt.Errorf("failed to derive w0: %w", err)
	}
	if _, err := io.ReadFull(hkdfReader, w1Bytes); err != nil {
		return nil, fmt.Errorf("failed to derive w1: %w", err)
	}

	w0 := new(big.Int).SetBytes(w0Bytes)
	w1 := new(big.Int).SetBytes(w1Bytes)
	w0.Mod(w0, curve.Params().N)
	w1.Mod(w1, curve.Params().N)

	return &SPAKE2PlusClient{
		setupCode:      setupCode,
		clientIdentity: clientIdentity,
		serverIdentity: serverIdentity,
		x:              x,
		w0:             w0,
		w1:             w1,
	}, nil
}

// PublicValue returns the client's public value (pA) to send to the server.
// pA = x*G + w0*M
func (c *SPAKE2PlusClient) PublicValue() []byte {
	if c.pA != nil {
		return c.pA
	}

	// X = x*G
	xx, xy := curve.ScalarBaseMult(c.x.Bytes())

	// w0*M
	w0mx, w0my := curve.ScalarMult(pointM.x, pointM.y, c.w0.Bytes())

	// pA = X + w0*M
	pAx, pAy := curve.Add(xx, xy, w0mx, w0my)

	c.pA = elliptic.Marshal(curve, pAx, pAy)
	return c.pA
}

// ProcessServerValue processes the server's public value and derives the shared secret.
func (c *SPAKE2PlusClient) ProcessServerValue(pB []byte) error {
	c.pB = pB

	// Ensure our public value is generated before deriving keys
	// (deriveKeys needs both pA and pB for the transcript)
	_ = c.PublicValue()

	// Parse pB
	pBx, pBy := elliptic.Unmarshal(curve, pB)
	if pBx == nil {
		return ErrInvalidPublicKey
	}

	// Verify pB is on the curve and not the identity
	if !curve.IsOnCurve(pBx, pBy) {
		return ErrInvalidPublicKey
	}

	// Y = pB - w0*N
	w0nx, w0ny := curve.ScalarMult(pointN.x, pointN.y, c.w0.Bytes())
	// Negate w0*N (invert y coordinate)
	w0nyNeg := new(big.Int).Neg(w0ny)
	w0nyNeg.Mod(w0nyNeg, curve.Params().P)

	yx, yy := curve.Add(pBx, pBy, w0nx, w0nyNeg)

	// Z = x * Y
	zx, zy := curve.ScalarMult(yx, yy, c.x.Bytes())

	// V = w1 * Y
	vx, vy := curve.ScalarMult(yx, yy, c.w1.Bytes())

	// Derive shared secrets using transcript
	c.deriveKeys(zx, zy, vx, vy)

	return nil
}

// deriveKeys derives the shared secret and confirmation keys.
func (c *SPAKE2PlusClient) deriveKeys(zx, zy, vx, vy *big.Int) {
	// Transcript: context || pA || pB || Z || V || w0
	h := sha256.New()
	h.Write(c.clientIdentity)
	h.Write(c.serverIdentity)
	h.Write(c.pA)
	h.Write(c.pB)
	h.Write(elliptic.Marshal(curve, zx, zy))
	h.Write(elliptic.Marshal(curve, vx, vy))
	h.Write(c.w0.Bytes())

	transcript := h.Sum(nil)

	// Derive keys using HKDF
	hkdfReader := hkdf.New(sha256.New, transcript, nil, []byte("SPAKE2+-P256-SHA256"))

	c.sharedSecret = make([]byte, SharedSecretSize)
	c.confirmKey = make([]byte, SharedSecretSize)
	io.ReadFull(hkdfReader, c.sharedSecret)
	io.ReadFull(hkdfReader, c.confirmKey)
}

// Confirmation returns the client's confirmation value.
func (c *SPAKE2PlusClient) Confirmation() []byte {
	mac := hmac.New(sha256.New, c.confirmKey)
	mac.Write([]byte("client"))
	mac.Write(c.pA)
	mac.Write(c.pB)
	return mac.Sum(nil)
}

// VerifyServerConfirmation verifies the server's confirmation value.
func (c *SPAKE2PlusClient) VerifyServerConfirmation(serverConfirm []byte) error {
	mac := hmac.New(sha256.New, c.confirmKey)
	mac.Write([]byte("server"))
	mac.Write(c.pB)
	mac.Write(c.pA)
	expected := mac.Sum(nil)

	if !hmac.Equal(serverConfirm, expected) {
		return ErrConfirmationFailed
	}
	return nil
}

// SharedSecret returns the derived shared secret.
// Only valid after ProcessServerValue has been called.
func (c *SPAKE2PlusClient) SharedSecret() []byte {
	return c.sharedSecret
}

// SPAKE2PlusServer represents the server side of the SPAKE2+ exchange.
type SPAKE2PlusServer struct {
	verifier       *Verifier
	serverIdentity []byte

	// Ephemeral private key
	y *big.Int

	// Exchange values
	pA []byte // Client's public value
	pB []byte // Our public value

	// Derived values from verifier
	w0     *big.Int
	lx, ly *big.Int

	// Shared secrets
	sharedSecret []byte
	confirmKey   []byte
}

// NewSPAKE2PlusServer creates a new SPAKE2+ server.
func NewSPAKE2PlusServer(verifier *Verifier, serverIdentity []byte) (*SPAKE2PlusServer, error) {
	if verifier == nil {
		return nil, ErrInvalidVerifier
	}

	// Generate ephemeral private key
	y, err := rand.Int(rand.Reader, curve.Params().N)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Parse w0 from verifier
	w0 := new(big.Int).SetBytes(verifier.W0)

	// Parse L from verifier
	lx, ly := elliptic.UnmarshalCompressed(curve, verifier.L)
	if lx == nil {
		return nil, fmt.Errorf("%w: invalid L point", ErrInvalidVerifier)
	}

	return &SPAKE2PlusServer{
		verifier:       verifier,
		serverIdentity: serverIdentity,
		y:              y,
		w0:             w0,
		lx:             lx,
		ly:             ly,
	}, nil
}

// PublicValue returns the server's public value (pB) to send to the client.
// pB = y*G + w0*N
func (s *SPAKE2PlusServer) PublicValue() []byte {
	if s.pB != nil {
		return s.pB
	}

	// Y = y*G
	yx, yy := curve.ScalarBaseMult(s.y.Bytes())

	// w0*N
	w0nx, w0ny := curve.ScalarMult(pointN.x, pointN.y, s.w0.Bytes())

	// pB = Y + w0*N
	pBx, pBy := curve.Add(yx, yy, w0nx, w0ny)

	s.pB = elliptic.Marshal(curve, pBx, pBy)
	return s.pB
}

// ProcessClientValue processes the client's public value and derives the shared secret.
func (s *SPAKE2PlusServer) ProcessClientValue(pA []byte) error {
	s.pA = pA

	// Ensure our public value is generated before deriving keys
	// (deriveKeys needs both pA and pB for the transcript)
	_ = s.PublicValue()

	// Parse pA
	pAx, pAy := elliptic.Unmarshal(curve, pA)
	if pAx == nil {
		return ErrInvalidPublicKey
	}

	// Verify pA is on the curve
	if !curve.IsOnCurve(pAx, pAy) {
		return ErrInvalidPublicKey
	}

	// X = pA - w0*M
	w0mx, w0my := curve.ScalarMult(pointM.x, pointM.y, s.w0.Bytes())
	// Negate w0*M (invert y coordinate)
	w0myNeg := new(big.Int).Neg(w0my)
	w0myNeg.Mod(w0myNeg, curve.Params().P)

	xx, xy := curve.Add(pAx, pAy, w0mx, w0myNeg)

	// Z = y * X
	zx, zy := curve.ScalarMult(xx, xy, s.y.Bytes())

	// V = y * L
	vx, vy := curve.ScalarMult(s.lx, s.ly, s.y.Bytes())

	// Derive shared secrets using transcript
	s.deriveKeys(zx, zy, vx, vy)

	return nil
}

// deriveKeys derives the shared secret and confirmation keys.
func (s *SPAKE2PlusServer) deriveKeys(zx, zy, vx, vy *big.Int) {
	// Transcript: context || pA || pB || Z || V || w0
	h := sha256.New()
	h.Write(s.verifier.Identity)
	h.Write(s.serverIdentity)
	h.Write(s.pA)
	h.Write(s.pB)
	h.Write(elliptic.Marshal(curve, zx, zy))
	h.Write(elliptic.Marshal(curve, vx, vy))
	h.Write(s.w0.Bytes())

	transcript := h.Sum(nil)

	// Derive keys using HKDF
	hkdfReader := hkdf.New(sha256.New, transcript, nil, []byte("SPAKE2+-P256-SHA256"))

	s.sharedSecret = make([]byte, SharedSecretSize)
	s.confirmKey = make([]byte, SharedSecretSize)
	io.ReadFull(hkdfReader, s.sharedSecret)
	io.ReadFull(hkdfReader, s.confirmKey)
}

// Confirmation returns the server's confirmation value.
func (s *SPAKE2PlusServer) Confirmation() []byte {
	mac := hmac.New(sha256.New, s.confirmKey)
	mac.Write([]byte("server"))
	mac.Write(s.pB)
	mac.Write(s.pA)
	return mac.Sum(nil)
}

// VerifyClientConfirmation verifies the client's confirmation value.
func (s *SPAKE2PlusServer) VerifyClientConfirmation(clientConfirm []byte) error {
	mac := hmac.New(sha256.New, s.confirmKey)
	mac.Write([]byte("client"))
	mac.Write(s.pA)
	mac.Write(s.pB)
	expected := mac.Sum(nil)

	if !hmac.Equal(clientConfirm, expected) {
		return ErrConfirmationFailed
	}
	return nil
}

// SharedSecret returns the derived shared secret.
// Only valid after ProcessClientValue has been called.
func (s *SPAKE2PlusServer) SharedSecret() []byte {
	return s.sharedSecret
}
