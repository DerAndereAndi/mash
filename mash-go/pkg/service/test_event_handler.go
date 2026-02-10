package service

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// RegisterTestEventHandler wires the triggerTestEvent command on the given
// TestControl feature to the device service's trigger dispatch. This must be
// called while the *TestControl wrapper is still available (before the inner
// Feature is added to the endpoint) and only when TestMode is enabled.
func (s *DeviceService) RegisterTestEventHandler(tc *features.TestControl) {
	tc.OnTriggerTestEvent(func(ctx context.Context, req features.TriggerTestEventRequest) error {
		// Validate enable key.
		if req.EnableKey != s.config.TestEnableKey {
			s.debugLog("triggerTestEvent: enable key mismatch")
			return fmt.Errorf("invalid enable key")
		}

		return s.dispatchTrigger(ctx, req.EventTrigger)
	})
}

// RegisterSetCommissioningWindowDurationHandler wires the setCommissioningWindowDuration
// command on the given TestControl feature. This allows the test harness to dynamically
// set the commissioning window duration on a running device.
func (s *DeviceService) RegisterSetCommissioningWindowDurationHandler(tc *features.TestControl) {
	tc.OnSetCommissioningWindowDuration(func(_ context.Context, req features.SetCommissioningWindowDurationRequest) error {
		// Validate enable key.
		if req.EnableKey != s.config.TestEnableKey {
			s.debugLog("setCommissioningWindowDuration: enable key mismatch")
			return fmt.Errorf("invalid enable key")
		}

		// Clamp to [3s, 10800s].
		durSec := req.DurationSeconds
		if durSec < 3 {
			durSec = 3
		}
		if durSec > 10800 {
			durSec = 10800
		}

		dur := time.Duration(durSec) * time.Second
		dm := s.DiscoveryManager()
		if dm != nil {
			dm.SetCommissioningWindowDuration(dur)
		}

		s.debugLog("setCommissioningWindowDuration: set",
			"requestedSeconds", req.DurationSeconds,
			"effectiveSeconds", durSec)
		return nil
	})
}

// dispatchTrigger routes a trigger opcode to the appropriate domain handler.
func (s *DeviceService) dispatchTrigger(ctx context.Context, trigger uint64) error {
	domain := features.TriggerDomain(trigger)

	switch domain {
	case uint16(model.FeatureDeviceInfo):
		return s.handleCommissioningTrigger(ctx, trigger)
	case uint16(model.FeatureStatus):
		return s.handleStatusTrigger(ctx, trigger)
	case uint16(model.FeatureMeasurement):
		return s.handleMeasurementTrigger(ctx, trigger)
	case uint16(model.FeatureChargingSession):
		return s.handleChargingSessionTrigger(ctx, trigger)
	case uint16(model.FeatureEnergyControl):
		return s.handleEnergyControlTrigger(ctx, trigger)
	default:
		return fmt.Errorf("unknown trigger domain: 0x%04x", domain)
	}
}

// handleCommissioningTrigger handles DeviceInfo-domain triggers.
func (s *DeviceService) handleCommissioningTrigger(_ context.Context, trigger uint64) error {
	switch trigger {
	case features.TriggerEnterCommissioningMode:
		s.debugLog("trigger: entering commissioning mode")
		return s.EnterCommissioningMode()
	case features.TriggerExitCommissioningMode:
		s.debugLog("trigger: exiting commissioning mode")
		return s.ExitCommissioningMode()
	case features.TriggerFactoryReset:
		s.debugLog("trigger: factory reset (removing all zones)")
		for _, zoneID := range s.ListZoneIDs() {
			_ = s.RemoveZone(zoneID)
		}
		return nil
	case features.TriggerResetTestState:
		s.debugLog("trigger: resetting test state to defaults")
		for _, ep := range s.device.Endpoints() {
			// Reset Status to STANDBY.
			if f, err := ep.GetFeatureByID(uint8(model.FeatureStatus)); err == nil {
				_ = f.SetAttributeInternal(features.StatusAttrOperatingState, uint8(features.OperatingStateStandby))
				_ = f.SetAttributeInternal(features.StatusAttrFaultCode, uint32(0))
				_ = f.SetAttributeInternal(features.StatusAttrFaultMessage, "")
			}
			// Reset Measurement to zero.
			if f, err := ep.GetFeatureByID(uint8(model.FeatureMeasurement)); err == nil {
				_ = f.SetAttributeInternal(features.MeasurementAttrACActivePower, int64(0))
			}
			// Reset ChargingSession to NOT_PLUGGED_IN.
			if f, err := ep.GetFeatureByID(uint8(model.FeatureChargingSession)); err == nil {
				_ = f.SetAttributeInternal(features.ChargingSessionAttrState,
					uint8(features.ChargingStateNotPluggedIn))
			}
			// Reset EnergyControl to AUTONOMOUS/NONE.
			if f, err := ep.GetFeatureByID(uint8(model.FeatureEnergyControl)); err == nil {
				_ = f.SetAttributeInternal(features.EnergyControlAttrControlState,
					uint8(features.ControlStateAutonomous))
				_ = f.SetAttributeInternal(features.EnergyControlAttrProcessState,
					uint8(features.ProcessStateNone))
			}
		}
		// Reset LimitResolver state (clears per-zone limits, timers, callbacks).
		if s.limitResolver != nil {
			s.limitResolver.ResetAll()
		}
		// Reset clock offset.
		s.clockOffset = 0
		// Reset commissioning window duration to configured default.
		if s.discoveryManager != nil && s.config.CommissioningWindowDuration > 0 {
			s.discoveryManager.SetCommissioningWindowDuration(s.config.CommissioningWindowDuration)
		}
		// Clear inbound subscriptions on all active zone sessions to stop
		// notifications from leaking into the next test.
		s.mu.Lock()
		for _, session := range s.zoneSessions {
			session.ClearSubscriptions()
		}
		s.mu.Unlock()
		return nil
	default:
		// Check for parameterized triggers (base + encoded value).
		if trigger&0xFFFF_FFFF_0000_0000 == features.TriggerAdjustClockBase {
			offsetSeconds := int32(trigger & 0xFFFF_FFFF)
			s.mu.Lock()
			s.clockOffset = time.Duration(offsetSeconds) * time.Second
			s.mu.Unlock()
			s.debugLog("trigger: clock offset adjusted", "offsetSeconds", offsetSeconds)
			return nil
		}
		return fmt.Errorf("unknown commissioning trigger: 0x%016x", trigger)
	}
}

// handleStatusTrigger handles Status-domain triggers.
func (s *DeviceService) handleStatusTrigger(_ context.Context, trigger uint64) error {
	for _, ep := range s.device.Endpoints() {
		feat, err := ep.GetFeatureByID(uint8(model.FeatureStatus))
		if err != nil {
			continue
		}

		switch trigger {
		case features.TriggerFault:
			s.debugLog("trigger: setting fault state", "endpoint", ep.ID())
			_ = feat.SetAttributeInternal(features.StatusAttrOperatingState, uint8(features.OperatingStateFault))
			_ = feat.SetAttributeInternal(features.StatusAttrFaultCode, uint32(1))
			_ = feat.SetAttributeInternal(features.StatusAttrFaultMessage, "Test trigger fault")
			return nil
		case features.TriggerClearFault:
			s.debugLog("trigger: clearing fault", "endpoint", ep.ID())
			_ = feat.SetAttributeInternal(features.StatusAttrOperatingState, uint8(features.OperatingStateRunning))
			_ = feat.SetAttributeInternal(features.StatusAttrFaultCode, uint32(0))
			_ = feat.SetAttributeInternal(features.StatusAttrFaultMessage, "")
			return nil
		case features.TriggerSetStandby:
			s.debugLog("trigger: setting standby", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.StatusAttrOperatingState, uint8(features.OperatingStateStandby))
		case features.TriggerSetRunning:
			s.debugLog("trigger: setting running", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.StatusAttrOperatingState, uint8(features.OperatingStateRunning))
		default:
			return fmt.Errorf("unknown status trigger: 0x%016x", trigger)
		}
	}
	return fmt.Errorf("no Status feature found on any endpoint")
}

// handleMeasurementTrigger handles Measurement-domain triggers.
func (s *DeviceService) handleMeasurementTrigger(_ context.Context, trigger uint64) error {
	for _, ep := range s.device.Endpoints() {
		feat, err := ep.GetFeatureByID(uint8(model.FeatureMeasurement))
		if err != nil {
			continue
		}

		switch {
		case trigger == features.TriggerSetPower100:
			s.debugLog("trigger: setting power to 100W", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrACActivePower, int64(100000))
		case trigger == features.TriggerSetPower1000:
			s.debugLog("trigger: setting power to 1kW", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrACActivePower, int64(1000000))
		case trigger == features.TriggerSetPowerZero:
			s.debugLog("trigger: setting power to 0W", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrACActivePower, int64(0))
		case trigger == features.TriggerSetSoC50:
			s.debugLog("trigger: setting SoC to 50%", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrStateOfCharge, uint8(50))
		case trigger == features.TriggerSetSoC100:
			s.debugLog("trigger: setting SoC to 100%", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrStateOfCharge, uint8(100))
		case trigger&0xFFFF_FFFF_0000_0000 == features.TriggerSetPowerCustomBase:
			// Custom power value encoded in lower 32 bits (milliwatts as int32).
			mw := int64(int32(trigger & 0xFFFF_FFFF))
			s.debugLog("trigger: setting power to custom value", "milliwatts", mw, "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.MeasurementAttrACActivePower, mw)
		default:
			return fmt.Errorf("unknown measurement trigger: 0x%016x", trigger)
		}
	}
	return fmt.Errorf("no Measurement feature found on any endpoint")
}

// handleChargingSessionTrigger handles ChargingSession-domain triggers.
func (s *DeviceService) handleChargingSessionTrigger(_ context.Context, trigger uint64) error {
	for _, ep := range s.device.Endpoints() {
		feat, err := ep.GetFeatureByID(uint8(model.FeatureChargingSession))
		if err != nil {
			continue
		}

		switch trigger {
		case features.TriggerEVPlugIn:
			s.debugLog("trigger: EV plug in", "endpoint", ep.ID())
			_ = feat.SetAttributeInternal(features.ChargingSessionAttrSessionStartTime,
				uint64(time.Now().Unix()))
			return feat.SetAttributeInternal(features.ChargingSessionAttrState,
				uint8(features.ChargingStatePluggedInNoDemand))
		case features.TriggerEVUnplug:
			s.debugLog("trigger: EV unplug", "endpoint", ep.ID())
			_ = feat.SetAttributeInternal(features.ChargingSessionAttrSessionEndTime,
				uint64(time.Now().Unix()))
			return feat.SetAttributeInternal(features.ChargingSessionAttrState,
				uint8(features.ChargingStateNotPluggedIn))
		case features.TriggerEVRequestCharge:
			s.debugLog("trigger: EV request charge", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.ChargingSessionAttrState,
				uint8(features.ChargingStatePluggedInDemand))
		default:
			return fmt.Errorf("unknown charging session trigger: 0x%016x", trigger)
		}
	}
	return fmt.Errorf("no ChargingSession feature found on any endpoint")
}

// handleEnergyControlTrigger handles EnergyControl-domain triggers.
func (s *DeviceService) handleEnergyControlTrigger(_ context.Context, trigger uint64) error {
	for _, ep := range s.device.Endpoints() {
		feat, err := ep.GetFeatureByID(uint8(model.FeatureEnergyControl))
		if err != nil {
			continue
		}

		switch trigger {
		// ControlState triggers.
		case features.TriggerControlStateAutonomous:
			s.debugLog("trigger: setting control state AUTONOMOUS", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrControlState,
				uint8(features.ControlStateAutonomous))
		case features.TriggerControlStateControlled:
			s.debugLog("trigger: setting control state CONTROLLED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrControlState,
				uint8(features.ControlStateControlled))
		case features.TriggerControlStateLimited:
			s.debugLog("trigger: setting control state LIMITED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrControlState,
				uint8(features.ControlStateLimited))
		case features.TriggerControlStateFailsafe:
			s.debugLog("trigger: setting control state FAILSAFE", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrControlState,
				uint8(features.ControlStateFailsafe))
		case features.TriggerControlStateOverride:
			s.debugLog("trigger: setting control state OVERRIDE", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrControlState,
				uint8(features.ControlStateOverride))

		// ProcessState triggers.
		case features.TriggerProcessStateNone:
			s.debugLog("trigger: setting process state NONE", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateNone))
		case features.TriggerProcessStateAvailable:
			s.debugLog("trigger: setting process state AVAILABLE", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateAvailable))
		case features.TriggerProcessStateScheduled:
			s.debugLog("trigger: setting process state SCHEDULED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateScheduled))
		case features.TriggerProcessStateRunning:
			s.debugLog("trigger: setting process state RUNNING", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateRunning))
		case features.TriggerProcessStatePaused:
			s.debugLog("trigger: setting process state PAUSED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStatePaused))
		case features.TriggerProcessStateCompleted:
			s.debugLog("trigger: setting process state COMPLETED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateCompleted))
		case features.TriggerProcessStateAborted:
			s.debugLog("trigger: setting process state ABORTED", "endpoint", ep.ID())
			return feat.SetAttributeInternal(features.EnergyControlAttrProcessState,
				uint8(features.ProcessStateAborted))
		default:
			return fmt.Errorf("unknown energy control trigger: 0x%016x", trigger)
		}
	}
	return fmt.Errorf("no EnergyControl feature found on any endpoint")
}
