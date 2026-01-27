// Package zone provides multi-zone management for MASH devices.
//
// MASH devices can belong to up to 5 zones simultaneously. Each zone represents
// a controller relationship (e.g., grid operator, home manager, user app).
//
// # Zone Types and Priority
//
// Zones have a type that determines their priority:
//
//   - GRID_OPERATOR (priority 1): Highest priority - DSO, smart meter gateway
//   - BUILDING_MANAGER (priority 2): Commercial building EMS
//   - HOME_MANAGER (priority 3): Residential energy management system
//   - USER_APP (priority 4): Lowest priority - mobile apps, voice assistants
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
