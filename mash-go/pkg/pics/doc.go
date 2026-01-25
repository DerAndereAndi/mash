// Package pics provides parsing and validation for MASH PICS
// (Protocol Implementation Conformance Statement) files.
//
// PICS files declare what features and capabilities a device or controller
// implements. The test harness uses PICS to select applicable tests.
//
// # PICS Code Format
//
// PICS codes follow this structure:
//
//	MASH.<Side>.<Feature>[.<Type><ID>][.<Qualifier>]
//
// Where:
//   - Side: S (Server/Device) or C (Client/Controller)
//   - Feature: Feature identifier (ELEC, MEAS, CTRL, etc.)
//   - Type: A (Attribute), C (Command), F (Feature flag), E (Event), B (Behavior)
//   - ID: Hex identifier
//   - Qualifier: Rsp (accepts/responds), Tx (generates/sends)
//
// # Example PICS File
//
//	# Device PICS
//	MASH.S=1
//	MASH.S.VERSION=1
//	MASH.S.CTRL=1
//	MASH.S.CTRL.A01=1       # deviceType
//	MASH.S.CTRL.C01.Rsp=1   # SetLimit
//
// # Validation
//
// The parser validates:
//   - Required protocol declaration (MASH.S or MASH.C)
//   - Feature flag dependencies (e.g., V2X requires EMOB)
//   - Attribute/command consistency with feature declarations
package pics
