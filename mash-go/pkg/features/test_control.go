package features

// Trigger opcode domain constants.
//
// The upper 2 bytes encode the feature domain (matching feature IDs from
// protocol-versions.yaml), the lower 2 bytes encode the specific trigger.
// Middle 4 bytes are reserved (zero).

// Commissioning triggers (domain 0x0001 = DeviceInfo).
const (
	TriggerEnterCommissioningMode uint64 = 0x0001_0000_0000_0001
	TriggerExitCommissioningMode  uint64 = 0x0001_0000_0000_0002
	TriggerFactoryReset           uint64 = 0x0001_0000_0000_0003
)

// Status triggers (domain 0x0002 = Status).
const (
	TriggerFault      uint64 = 0x0002_0000_0000_0001
	TriggerClearFault uint64 = 0x0002_0000_0000_0002
	TriggerSetStandby uint64 = 0x0002_0000_0000_0003
	TriggerSetRunning uint64 = 0x0002_0000_0000_0004
)

// Measurement triggers (domain 0x0004 = Measurement).
const (
	TriggerSetPower100  uint64 = 0x0004_0000_0000_0001 // 100W
	TriggerSetPower1000 uint64 = 0x0004_0000_0000_0002 // 1kW
	TriggerSetPowerZero uint64 = 0x0004_0000_0000_0003
	TriggerSetSoC50     uint64 = 0x0004_0000_0000_0010
	TriggerSetSoC100    uint64 = 0x0004_0000_0000_0011
)

// ChargingSession triggers (domain 0x0006 = ChargingSession).
const (
	TriggerEVPlugIn        uint64 = 0x0006_0000_0000_0001
	TriggerEVUnplug        uint64 = 0x0006_0000_0000_0002
	TriggerEVRequestCharge uint64 = 0x0006_0000_0000_0003
)

// TriggerTestEvent command parameter and response keys.
const (
	TriggerTestEventParamEnableKey    = "enableKey"
	TriggerTestEventParamEventTrigger = "eventTrigger"
	TriggerTestEventRespSuccess       = "success"
)

// TriggerDomain extracts the feature domain from a trigger opcode.
// The domain is encoded in the upper 2 bytes.
func TriggerDomain(trigger uint64) uint16 {
	return uint16(trigger >> 48)
}
