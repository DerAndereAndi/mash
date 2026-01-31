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
// # Use Case PICS Codes
//
// Use case codes declare which use cases a device or controller supports:
//
//	MASH.S.UC.<name>=1      # Device implements use case (server-side)
//	MASH.C.UC.<name>=1      # Controller supports use case (client-side)
//
// Examples:
//
//	MASH.S.UC.LPC=1         # Device supports Limit Power Consumption
//	MASH.S.UC.EVC=1         # Device supports EV Charging
//	MASH.C.UC.LPC=1         # Controller supports LPC
//	MASH.C.UC.MPD=1         # Controller supports Monitor Power Device
//
// UC codes are device-level (no endpoint prefix). The endpoint mapping is
// carried by the UseCaseDecl wire structure; the PICS code simply declares
// "supported or not" for test filtering.
//
// Use [PICS.HasUseCase] and [PICS.UseCases] for convenient access.
// Use [GenerateUseCaseCodes] to produce UC entries from wire declarations.
//
// # Example PICS File
//
//	# Device PICS
//	MASH.S=1
//	MASH.S.VERSION=1
//	MASH.S.UC.LPC=1         # Use case declaration
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
