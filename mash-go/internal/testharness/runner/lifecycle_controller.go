package runner

import (
	"fmt"
	"sync"
)

// LifecycleState tracks the strict control-channel lifecycle state.
type LifecycleState int

const (
	LifecycleDisconnected LifecycleState = iota
	LifecycleCommissioning
	LifecycleOperational
)

// LifecycleController enforces deterministic lifecycle transitions and a
// single active control-channel authority.
type LifecycleController struct {
	mu        sync.Mutex
	state     LifecycleState
	authority string
}

func NewLifecycleController() *LifecycleController {
	return &LifecycleController{state: LifecycleDisconnected}
}

func (c *LifecycleController) State() LifecycleState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *LifecycleController) Authority() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authority
}

func (c *LifecycleController) ToCommissioning(authority string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if authority == "" {
		return fmt.Errorf("lifecycle: commissioning requires authority")
	}
	switch c.state {
	case LifecycleDisconnected:
		c.state = LifecycleCommissioning
		c.authority = authority
		return nil
	case LifecycleCommissioning:
		if c.authority != authority {
			return fmt.Errorf("lifecycle: authority conflict: %q already active, cannot switch to %q", c.authority, authority)
		}
		return nil
	default:
		return fmt.Errorf("lifecycle: invalid transition %v -> commissioning", c.state)
	}
}

func (c *LifecycleController) ToOperational(authority string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if authority == "" {
		return fmt.Errorf("lifecycle: operational requires authority")
	}
	if c.state != LifecycleCommissioning {
		return fmt.Errorf("lifecycle: invalid transition %v -> operational", c.state)
	}
	c.state = LifecycleOperational
	c.authority = authority
	return nil
}

// ReconnectOperational supports re-establishing an operational control channel
// from disconnected (or replacing an existing operational channel).
func (c *LifecycleController) ReconnectOperational(authority string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if authority == "" {
		return fmt.Errorf("lifecycle: operational reconnect requires authority")
	}
	switch c.state {
	case LifecycleDisconnected:
		c.state = LifecycleOperational
		c.authority = authority
		return nil
	case LifecycleCommissioning:
		// Teardown/recovery may need to recover an operational suite channel
		// from a stale commissioning state marker.
		c.state = LifecycleOperational
		c.authority = authority
		return nil
	case LifecycleOperational:
		c.authority = authority
		return nil
	default:
		return fmt.Errorf("lifecycle: invalid transition %v -> operational(reconnect)", c.state)
	}
}

func (c *LifecycleController) ToDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = LifecycleDisconnected
	c.authority = ""
}
