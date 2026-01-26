package connection

import (
	"math/rand"
	"sync"
	"time"
)

// Backoff constants from the MASH transport specification.
const (
	// InitialBackoff is the initial reconnection delay.
	InitialBackoff = 1 * time.Second

	// MaxBackoff is the maximum reconnection delay.
	MaxBackoff = 60 * time.Second

	// BackoffMultiplier is the factor by which backoff increases.
	BackoffMultiplier = 2.0

	// JitterFactor is the maximum jitter as a fraction of base delay.
	JitterFactor = 0.25
)

// Backoff calculates exponential backoff delays with jitter.
type Backoff struct {
	mu sync.Mutex

	// Current backoff delay (before jitter)
	current time.Duration

	// Configuration
	initial    time.Duration
	max        time.Duration
	multiplier float64
	jitter     float64

	// Attempt counter
	attempts int

	// Random source for jitter
	rng *rand.Rand
}

// NewBackoff creates a new backoff calculator with default settings.
func NewBackoff() *Backoff {
	return &Backoff{
		current:    InitialBackoff,
		initial:    InitialBackoff,
		max:        MaxBackoff,
		multiplier: BackoffMultiplier,
		jitter:     JitterFactor,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// BackoffConfig allows customizing backoff parameters.
type BackoffConfig struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
	Jitter     float64
}

// NewBackoffWithConfig creates a backoff calculator with custom settings.
func NewBackoffWithConfig(cfg BackoffConfig) *Backoff {
	if cfg.Initial <= 0 {
		cfg.Initial = InitialBackoff
	}
	if cfg.Max <= 0 {
		cfg.Max = MaxBackoff
	}
	if cfg.Multiplier <= 1 {
		cfg.Multiplier = BackoffMultiplier
	}
	if cfg.Jitter < 0 {
		cfg.Jitter = 0
	}

	return &Backoff{
		current:    cfg.Initial,
		initial:    cfg.Initial,
		max:        cfg.Max,
		multiplier: cfg.Multiplier,
		jitter:     cfg.Jitter,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Next returns the next backoff delay (with jitter) and advances the backoff.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate jittered delay
	delay := b.addJitter(b.current)

	// Advance to next backoff value
	b.attempts++
	next := time.Duration(float64(b.current) * b.multiplier)
	if next > b.max {
		next = b.max
	}
	b.current = next

	return delay
}

// Peek returns the current backoff delay without advancing.
func (b *Backoff) Peek() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.addJitter(b.current)
}

// Reset resets the backoff to initial values.
// Call this after a successful connection.
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = b.initial
	b.attempts = 0
}

// Attempts returns the number of backoff attempts since last reset.
func (b *Backoff) Attempts() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempts
}

// Current returns the current base backoff (without jitter).
func (b *Backoff) Current() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current
}

// addJitter adds random jitter to a delay.
func (b *Backoff) addJitter(d time.Duration) time.Duration {
	if b.jitter <= 0 {
		return d
	}
	jitterAmount := time.Duration(float64(d) * b.jitter * b.rng.Float64())
	return d + jitterAmount
}

// BackoffSequence returns the sequence of base backoff values
// (without jitter) up to the maximum.
func BackoffSequence() []time.Duration {
	return []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		60 * time.Second, // max
	}
}
