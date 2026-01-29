package examples

import (
	"context"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// EVSE represents a complete EV charger device implementation.
// It demonstrates how to build a controllable MASH device that:
//   - Exposes its capabilities via Electrical feature
//   - Reports measurements via Measurement feature
//   - Accepts control via EnergyControl feature
//   - Manages charging sessions via ChargingSession feature
//   - Reports health via Status feature
type EVSE struct {
	mu sync.RWMutex

	device *model.Device

	// Features
	deviceInfo      *features.DeviceInfo
	electrical      *features.Electrical
	measurement     *features.Measurement
	energyControl   *features.EnergyControl
	chargingSession *features.ChargingSession
	status          *features.Status

	// Internal state
	currentPower     int64 // mW - actual charging power
	sessionCounter   uint32
	effectiveLimit   *int64 // mW - active power limit from CEM

	// Callbacks for external integration
	onLimitChanged func(limit *int64)
}

// EVSEConfig contains configuration for creating an EVSE.
type EVSEConfig struct {
	DeviceID     string
	VendorName   string
	ProductName  string
	SerialNumber string
	VendorID     uint32
	ProductID    uint32

	// Electrical capabilities
	PhaseCount            uint8
	NominalVoltage        uint16 // V
	MaxCurrentPerPhase    int64  // mA
	MinCurrentPerPhase    int64  // mA
	NominalMaxPower       int64  // mW
	NominalMinPower       int64  // mW
	SupportsBidirectional bool
}

// NewEVSE creates a new EVSE device with the given configuration.
func NewEVSE(cfg EVSEConfig) *EVSE {
	evse := &EVSE{
		sessionCounter: 1,
	}

	// Create device
	evse.device = model.NewDevice(cfg.DeviceID, cfg.VendorID, cfg.ProductID)

	// Setup root endpoint with DeviceInfo
	evse.setupDeviceInfo(cfg)

	// Setup charger endpoint with all features
	evse.setupChargerEndpoint(cfg)

	// Wire up command handlers
	evse.setupCommandHandlers()

	return evse
}

func (e *EVSE) setupDeviceInfo(cfg EVSEConfig) {
	e.deviceInfo = features.NewDeviceInfo()
	// Device-level capability bitmap indicates overall device type
	e.deviceInfo.Feature.SetFeatureMap(uint32(model.FeatureMapCore | model.FeatureMapFlex | model.FeatureMapEMob))
	_ = e.deviceInfo.SetDeviceID(cfg.DeviceID)
	_ = e.deviceInfo.SetVendorName(cfg.VendorName)
	_ = e.deviceInfo.SetProductName(cfg.ProductName)
	_ = e.deviceInfo.SetSerialNumber(cfg.SerialNumber)
	_ = e.deviceInfo.SetVendorID(cfg.VendorID)
	_ = e.deviceInfo.SetProductID(cfg.ProductID)
	_ = e.deviceInfo.SetSoftwareVersion("1.0.0")

	e.device.RootEndpoint().AddFeature(e.deviceInfo.Feature)
}

func (e *EVSE) setupChargerEndpoint(cfg EVSEConfig) {
	charger := model.NewEndpoint(1, model.EndpointEVCharger, "EV Charger")

	// EVSE capability bitmap: CORE + FLEX + EMOB
	evseCapabilities := uint32(model.FeatureMapCore | model.FeatureMapFlex | model.FeatureMapEMob)

	// Electrical - static capabilities
	e.electrical = features.NewElectrical()
	e.electrical.Feature.SetFeatureMap(evseCapabilities)
	_ = e.electrical.SetPhaseCount(cfg.PhaseCount)
	_ = e.electrical.SetNominalVoltage(cfg.NominalVoltage)
	_ = e.electrical.SetNominalFrequency(50)
	_ = e.electrical.SetMaxCurrentPerPhase(cfg.MaxCurrentPerPhase)
	_ = e.electrical.SetMinCurrentPerPhase(cfg.MinCurrentPerPhase)
	_ = e.electrical.SetNominalMaxConsumption(cfg.NominalMaxPower)
	_ = e.electrical.SetNominalMinPower(cfg.NominalMinPower)

	direction := features.DirectionConsumption
	if cfg.SupportsBidirectional {
		direction = features.DirectionBidirectional
	}
	_ = e.electrical.SetSupportedDirections(direction)
	charger.AddFeature(e.electrical.Feature)

	// Measurement - real-time telemetry
	e.measurement = features.NewMeasurement()
	e.measurement.Feature.SetFeatureMap(evseCapabilities)
	charger.AddFeature(e.measurement.Feature)

	// EnergyControl - accepts limits from CEM
	e.energyControl = features.NewEnergyControl()
	e.energyControl.Feature.SetFeatureMap(evseCapabilities)
	_ = e.energyControl.SetDeviceType(features.DeviceTypeEVSE)
	_ = e.energyControl.SetControlState(features.ControlStateAutonomous)
	e.energyControl.SetCapabilities(
		true,  // acceptsLimits
		true,  // acceptsCurrentLimits
		false, // acceptsSetpoints
		false, // acceptsCurrentSetpoints
		true,  // isPausable
		false, // isShiftable
		true,  // isStoppable
	)
	charger.AddFeature(e.energyControl.Feature)

	// ChargingSession - EV session management
	e.chargingSession = features.NewChargingSession()
	e.chargingSession.Feature.SetFeatureMap(evseCapabilities)
	_ = e.chargingSession.SetSupportedChargingModes([]features.ChargingMode{
		features.ChargingModeOff,
		features.ChargingModePVSurplusOnly,
		features.ChargingModePVSurplusThreshold,
		features.ChargingModePriceOptimized,
	})
	charger.AddFeature(e.chargingSession.Feature)

	// Status - health and operating state
	e.status = features.NewStatus()
	e.status.Feature.SetFeatureMap(evseCapabilities)
	_ = e.status.SetOperatingState(features.OperatingStateStandby)
	charger.AddFeature(e.status.Feature)

	_ = e.device.AddEndpoint(charger)

	// Update DeviceInfo with endpoint structure
	e.updateEndpointInfo()
}

func (e *EVSE) setupCommandHandlers() {
	// SetLimit handler - CEM sets power limits
	e.energyControl.OnSetLimitEnhanced(func(ctx context.Context, req features.SetLimitRequest) features.SetLimitResponse {
		e.mu.Lock()
		defer e.mu.Unlock()

		// Validate: reject negative limits
		if req.ConsumptionLimit != nil && *req.ConsumptionLimit < 0 {
			reason := features.LimitRejectInvalidValue
			return features.SetLimitResponse{
				Applied:      false,
				RejectReason: &reason,
				ControlState: e.energyControl.ControlState(),
			}
		}

		// Store the limit
		e.effectiveLimit = req.ConsumptionLimit

		// Update control state based on whether limit is set
		var newState features.ControlState
		if req.ConsumptionLimit != nil {
			newState = features.ControlStateLimited
			_ = e.energyControl.SetEffectiveConsumptionLimit(req.ConsumptionLimit)
		} else {
			newState = features.ControlStateControlled
			_ = e.energyControl.SetEffectiveConsumptionLimit(nil)
		}
		_ = e.energyControl.SetControlState(newState)

		// Notify callback
		if e.onLimitChanged != nil {
			e.onLimitChanged(req.ConsumptionLimit)
		}

		return features.SetLimitResponse{
			Applied:                   true,
			EffectiveConsumptionLimit: req.ConsumptionLimit,
			EffectiveProductionLimit:  req.ProductionLimit,
			ControlState:              newState,
		}
	})

	// ClearLimit handler
	e.energyControl.OnClearLimit(func(ctx context.Context, direction *features.Direction) error {
		e.mu.Lock()
		defer e.mu.Unlock()

		e.effectiveLimit = nil
		_ = e.energyControl.SetControlState(features.ControlStateControlled)
		_ = e.energyControl.SetEffectiveConsumptionLimit(nil)

		if e.onLimitChanged != nil {
			e.onLimitChanged(nil)
		}

		return nil
	})

	// Pause handler
	e.energyControl.OnPause(func(ctx context.Context, duration *uint32) error {
		e.mu.Lock()
		defer e.mu.Unlock()

		_ = e.status.SetOperatingState(features.OperatingStatePaused)
		_ = e.energyControl.SetProcessState(features.ProcessStatePaused)
		e.currentPower = 0
		_ = e.measurement.SetACActivePower(0)

		return nil
	})

	// Resume handler
	e.energyControl.OnResume(func(ctx context.Context) error {
		e.mu.Lock()
		defer e.mu.Unlock()

		if e.chargingSession.IsPluggedIn() {
			_ = e.status.SetOperatingState(features.OperatingStateRunning)
			_ = e.energyControl.SetProcessState(features.ProcessStateRunning)
		}

		return nil
	})

	// Stop handler
	e.energyControl.OnStop(func(ctx context.Context) error {
		e.mu.Lock()
		defer e.mu.Unlock()

		_ = e.energyControl.SetProcessState(features.ProcessStateAborted)
		e.currentPower = 0
		_ = e.measurement.SetACActivePower(0)

		return nil
	})

	// SetChargingMode handler
	e.chargingSession.OnSetChargingMode(func(ctx context.Context, mode features.ChargingMode, surplusThreshold *int64, startDelay, stopDelay *uint32) (features.ChargingMode, string, error) {
		// Check if mode is supported
		if !e.chargingSession.SupportsMode(mode) {
			return e.chargingSession.CurrentChargingMode(), "unsupported mode", nil
		}

		_ = e.chargingSession.SetChargingMode(mode)
		if surplusThreshold != nil {
			_ = e.chargingSession.SetSurplusThreshold(*surplusThreshold)
		}
		if startDelay != nil {
			_ = e.chargingSession.SetStartDelay(*startDelay)
		}
		if stopDelay != nil {
			_ = e.chargingSession.SetStopDelay(*stopDelay)
		}

		return mode, "", nil
	})
}

func (e *EVSE) updateEndpointInfo() {
	endpoints := make([]*model.EndpointInfo, 0)
	for _, ep := range e.device.Endpoints() {
		endpoints = append(endpoints, ep.Info())
	}
	_ = e.deviceInfo.SetEndpoints(endpoints)
}

// Device returns the underlying MASH device.
func (e *EVSE) Device() *model.Device {
	return e.device
}

// Simulation methods for testing

// SimulateEVConnect simulates an EV connecting to the charger.
func (e *EVSE) SimulateEVConnect(evSoC uint8, evCapacity uint64, demandMode features.EVDemandMode) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sessionID := e.sessionCounter
	e.sessionCounter++

	_ = e.chargingSession.StartSession(sessionID, uint64(time.Now().Unix()))
	_ = e.chargingSession.SetEVStateOfCharge(evSoC)
	_ = e.chargingSession.SetEVBatteryCapacity(evCapacity)
	_ = e.chargingSession.SetEVDemandMode(demandMode)

	// Calculate energy requests based on SoC
	currentEnergy := int64(evCapacity) * int64(evSoC) / 100
	maxEnergy := int64(evCapacity) - currentEnergy
	targetEnergy := int64(evCapacity)*80/100 - currentEnergy // Target 80%

	_ = e.chargingSession.SetEVEnergyRequests(nil, &maxEnergy, &targetEnergy)
	_ = e.chargingSession.SetState(features.ChargingStatePluggedInDemand)

	_ = e.status.SetOperatingState(features.OperatingStateRunning)
	_ = e.energyControl.SetProcessState(features.ProcessStateRunning)
}

// SimulateEVDisconnect simulates the EV disconnecting.
func (e *EVSE) SimulateEVDisconnect() {
	e.mu.Lock()
	defer e.mu.Unlock()

	_ = e.chargingSession.EndSession(uint64(time.Now().Unix()))
	_ = e.chargingSession.ClearEVStateOfCharge()
	_ = e.chargingSession.ClearEVBatteryCapacity()

	e.currentPower = 0
	_ = e.measurement.SetACActivePower(0)
	_ = e.status.SetOperatingState(features.OperatingStateStandby)
	_ = e.energyControl.SetProcessState(features.ProcessStateNone)
}

// SimulateCharging simulates active charging at the given power.
func (e *EVSE) SimulateCharging(power int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Apply limit if set
	if e.effectiveLimit != nil && power > *e.effectiveLimit {
		power = *e.effectiveLimit
	}

	e.currentPower = power
	_ = e.measurement.SetACActivePower(power)
	_ = e.chargingSession.SetState(features.ChargingStatePluggedInCharging)
}

// GetCurrentPower returns the current charging power in mW.
func (e *EVSE) GetCurrentPower() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentPower
}

// GetEffectiveLimit returns the current effective limit from CEM.
func (e *EVSE) GetEffectiveLimit() *int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.effectiveLimit == nil {
		return nil
	}
	limit := *e.effectiveLimit
	return &limit
}

// OnLimitChanged sets a callback for when the CEM changes limits.
func (e *EVSE) OnLimitChanged(handler func(limit *int64)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onLimitChanged = handler
}

// AcceptController marks the EVSE as being controlled.
func (e *EVSE) AcceptController() {
	e.mu.Lock()
	defer e.mu.Unlock()
	_ = e.energyControl.SetControlState(features.ControlStateControlled)
}
