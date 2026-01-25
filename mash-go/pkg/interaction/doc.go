// Package interaction implements the MASH interaction model.
//
// The interaction model defines four operations between controllers and devices:
//
//   - Read: Get current attribute values
//   - Write: Set attribute values (full replacement)
//   - Subscribe: Register for change notifications
//   - Invoke: Execute commands with parameters
//
// # Server Usage
//
// The Server handles incoming requests and dispatches them to the device model:
//
//	device := model.NewDevice("device-123", vendorID, productID)
//	server := interaction.NewServer(device)
//
//	// Handle incoming request
//	response := server.HandleRequest(ctx, request)
//
//	// The server also manages subscriptions and sends notifications
//	server.SetNotificationHandler(func(notif *wire.Notification) {
//	    // Send notification to client
//	})
//
// # Client Usage
//
// The Client provides a high-level API for making requests:
//
//	client := interaction.NewClient(conn)
//
//	// Read attributes
//	values, err := client.Read(ctx, endpointID, featureID, []uint16{1, 2, 3})
//
//	// Write attributes
//	err = client.Write(ctx, endpointID, featureID, map[uint16]any{21: 6000000})
//
//	// Subscribe to changes
//	subID, initial, err := client.Subscribe(ctx, endpointID, featureID, nil)
//
//	// Invoke command
//	result, err := client.Invoke(ctx, endpointID, featureID, cmdID, params)
//
// # Subscription Management
//
// Subscriptions have three key behaviors:
//
//  1. Priming Report: Initial response contains current values of all subscribed attributes
//  2. Delta Notifications: Only changed attributes are sent in subsequent notifications
//  3. Heartbeat: If no changes occur within maxInterval, current values are sent
//
// Subscriptions are connection-scoped. When a connection is closed, all subscriptions
// for that connection are automatically cleaned up.
package interaction
