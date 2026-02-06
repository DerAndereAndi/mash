package runner

import (
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
)

// discoveryState holds state for mDNS discovery handlers.
type discoveryState struct {
	services  []discoveredService
	browser   any // *discovery.Browser when real impl used
	active    bool
	qrPayload string
}

// discoveredService represents a discovered mDNS service.
type discoveredService struct {
	InstanceName  string
	Host          string
	Port          uint16
	Addresses     []string
	ServiceType   string
	TXTRecords    map[string]string
	Discriminator uint16
}

// getDiscoveryState retrieves or creates discovery state from execution state.
func getDiscoveryState(state *engine.ExecutionState) *discoveryState {
	if s, ok := state.Custom["discovery"].(*discoveryState); ok {
		return s
	}
	s := &discoveryState{
		services: make([]discoveredService, 0),
	}
	state.Custom["discovery"] = s
	return s
}

// zoneState holds state for zone management handlers.
type zoneState struct {
	zones     map[string]*zoneInfo
	zoneOrder []string
	maxZones  int
}

// zoneInfo represents a single zone.
type zoneInfo struct {
	ZoneID           string
	ZoneName         string
	ZoneType         string
	Priority         int
	Metadata         map[string]any
	CAFingerprint    string
	Connected        bool
	DeviceIDs        []string
	CommissionedAt   time.Time
	LastSeen         time.Time
	LastSeenUpdated  bool // tracks whether LastSeen changed on last operation
}

// zonePriority maps zone types to their priority (higher number = higher priority).
var zonePriority = map[string]int{
	ZoneTypeGrid:  2,
	ZoneTypeLocal: 1,
	ZoneTypeTest:         0, // Lowest priority -- observer only (DEC-060)
}

// getZoneState retrieves or creates zone state from execution state.
func getZoneState(state *engine.ExecutionState) *zoneState {
	if s, ok := state.Custom["zones"].(*zoneState); ok {
		return s
	}
	s := &zoneState{
		zones:    make(map[string]*zoneInfo),
		maxZones: 5,
	}
	state.Custom["zones"] = s
	return s
}

// deviceState holds state for device simulation handlers.
type deviceState struct {
	operatingState string
	controlState   string
	processState   string
	faults         []faultEntry
	stateDetails   map[string]any
	failsafeLimit  *float64
	evConnected    bool
	cablePluggedIn bool
	configured     bool
	attributes     map[string]any
}

// faultEntry represents an active fault.
type faultEntry struct {
	Code    uint32
	Message string
	Time    time.Time
}

// getDeviceState retrieves or creates device state from execution state.
func getDeviceState(state *engine.ExecutionState) *deviceState {
	if s, ok := state.Custom["device_state"].(*deviceState); ok {
		return s
	}
	s := &deviceState{
		operatingState: OperatingStateStandby,
		controlState:   ControlStateAutonomous,
		processState:   ProcessStateNone,
		faults:         make([]faultEntry, 0),
		stateDetails:   make(map[string]any),
		attributes:     make(map[string]any),
	}
	state.Custom["device_state"] = s
	return s
}

// connectionTracker holds state for zone-scoped connections and command queuing.
type connectionTracker struct {
	zoneConnections map[string]*Connection
	pendingQueue    []queuedCommand
	backoffState    *backoffTracker
	clockOffset     time.Duration
}

// queuedCommand is a command waiting to be sent.
type queuedCommand struct {
	Action string
	Params map[string]any
}

// backoffTracker tracks reconnection backoff state.
type backoffTracker struct {
	Attempts    int
	LastAttempt time.Time
	Monitoring  bool
}

// getConnectionTracker retrieves or creates connection tracker from execution state.
func getConnectionTracker(state *engine.ExecutionState) *connectionTracker {
	if s, ok := state.Custom["connections"].(*connectionTracker); ok {
		return s
	}
	s := &connectionTracker{
		zoneConnections: make(map[string]*Connection),
		pendingQueue:    make([]queuedCommand, 0),
	}
	state.Custom["connections"] = s
	return s
}

// controllerState holds state for controller action handlers.
type controllerState struct {
	controllerID                string
	commissioningWindowDuration time.Duration
	devices                     map[string]string // deviceID -> zoneID
}

// getControllerState retrieves or creates controller state from execution state.
func getControllerState(state *engine.ExecutionState) *controllerState {
	if s, ok := state.Custom["controller_state"].(*controllerState); ok {
		return s
	}
	s := &controllerState{
		commissioningWindowDuration: 15 * time.Minute,
		devices:                     make(map[string]string),
	}
	state.Custom["controller_state"] = s
	return s
}
