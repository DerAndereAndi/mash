// Package zone provides multi-zone management for MASH devices.
//
// MASH devices can belong to up to 5 zones simultaneously. Each zone represents
// a controller relationship (e.g., grid operator, local energy manager).
//
// # Zone Types and Priority
//
// Zones have a type that determines their priority:
//
//   - GRID (priority 1): External/regulatory authority - DSO, smart meter gateway, aggregators
//   - LOCAL (priority 2): Local energy management - EMS (residential or commercial)
//
// # Priority Resolution
//
// When multiple zones set values, resolution depends on the type:
//
//   - Limits: Most restrictive wins (smallest value)
//   - Setpoints: Highest priority wins (lowest priority number)
//
// # Zone Lifecycle
//
//   - Addition: Via commissioning (SPAKE2+ + certificate issuance)
//   - Removal: Via RemoveZone command (self-removal only)
//   - Maximum: 5 zones per device
//
// The [Manager] coordinates zone membership, value resolution, and lifecycle.
package zone
