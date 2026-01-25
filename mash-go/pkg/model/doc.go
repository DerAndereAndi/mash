// Package model implements the MASH device data model.
//
// # Device Model Hierarchy
//
// MASH uses a 3-level hierarchy inspired by Matter:
//
//	Device > Endpoint > Feature
//
// A Device represents a physical or logical device (e.g., an EVSE, inverter).
// Devices contain one or more Endpoints, each representing a functional unit.
// Endpoints contain Features, which group related attributes and commands.
//
// # Endpoint Structure
//
// Every device has at least Endpoint 0 (DEVICE_ROOT) with the DeviceInfo feature.
// Additional endpoints represent functional capabilities:
//
//	Device (evse-001)
//	├── Endpoint 0 (DEVICE_ROOT)
//	│   └── DeviceInfo
//	├── Endpoint 1 (EV_CHARGER)
//	│   ├── Electrical
//	│   ├── Measurement
//	│   ├── EnergyControl
//	│   └── ChargingSession
//	└── ...
//
// # Features
//
// Features organize device capabilities by concern:
//   - Electrical: "What CAN this do?" (static configuration)
//   - Measurement: "What IS it doing?" (telemetry)
//   - EnergyControl: "What SHOULD it do?" (control)
//   - Status: "Is it working?" (operating state)
//
// Each feature has:
//   - Attributes: Data values with metadata (type, access, constraints)
//   - Commands: Invokable operations with parameters
//   - Global attributes: clusterRevision, featureMap, attributeList
//
// # Addressing
//
// Resources are addressed by the tuple:
//
//	(EndpointID, FeatureID, AttributeID) for attributes
//	(EndpointID, FeatureID, CommandID) for commands
//
// # Access Control
//
// Attributes have access flags:
//   - Read: Can be read
//   - Write: Can be written
//   - Subscribe: Can be subscribed for changes
//
// Commands have access flags:
//   - Invoke: Can be invoked
//
// Zone membership determines which controllers can access resources.
package model
