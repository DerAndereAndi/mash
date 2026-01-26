// Package persistence provides runtime state persistence for MASH devices and controllers.
//
// This package handles the JSON serialization of runtime state (zone memberships,
// failsafe timer snapshots, zone index mappings) that must survive device restarts.
// Certificate storage is handled separately by the cert package's FileStore.
package persistence
