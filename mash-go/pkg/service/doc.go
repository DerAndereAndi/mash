// Package service provides high-level orchestration for MASH devices and controllers.
//
// This package ties together all the lower-level components into cohesive APIs
// for building complete MASH implementations:
//
// # DeviceService
//
// DeviceService orchestrates a MASH device. It handles:
//   - mDNS advertising (commissionable and operational)
//   - Incoming WebSocket connections
//   - PASE commissioning for new zones
//   - Request dispatch to the device model
//   - Subscription notifications
//   - Failsafe and duration timer management
//
// Example usage:
//
//	device := model.NewDevice(...)
//	config := service.DefaultDeviceConfig()
//	config.SetupCode = "12345678"
//
//	svc, err := service.NewDeviceService(device, config)
//	svc.Start(ctx)
//	defer svc.Stop()
//
// # ControllerService
//
// ControllerService orchestrates a MASH controller (EMS). It handles:
//   - mDNS browsing for devices
//   - Outgoing WebSocket connections
//   - PASE commissioning of new devices
//   - Request/response handling
//   - Subscription management
//   - Multi-device coordination
//
// Example usage:
//
//	config := service.DefaultControllerConfig()
//	ctrl, err := service.NewControllerService(config)
//	ctrl.Start(ctx)
//	defer ctrl.Stop()
//
//	// Discover and commission a device
//	devices, _ := ctrl.Discover(ctx, discovery.FilterByCategory(discovery.CategoryEMobility))
//	device, _ := ctrl.Commission(ctx, devices[0], "12345678")
//
//	// Interact with device
//	values, _ := device.Read(ctx, 1, features.EnergyControlID, nil)
//
// # Connection Lifecycle
//
// Both services implement the connection lifecycle:
//   - Automatic reconnection with exponential backoff
//   - Heartbeat/keep-alive monitoring
//   - Failsafe timer management
//   - Graceful shutdown
//
// # Event Callbacks
//
// Services emit events for important state changes:
//   - OnConnect/OnDisconnect: Connection state changes
//   - OnCommission/OnDecommission: Zone membership changes
//   - OnValueChange: Attribute value changes
//   - OnFailsafe: Failsafe timer events
package service
