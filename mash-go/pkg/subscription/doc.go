// Package subscription implements subscription management for MASH devices.
//
// MASH subscriptions allow controllers to receive notifications when attribute
// values change. The subscription system handles coalescing, heartbeats, and
// bounce-back suppression.
//
// # Subscription Parameters
//
// Each subscription has:
//   - minInterval: Minimum seconds between notifications (coalescing window)
//   - maxInterval: Maximum seconds without notification (heartbeat)
//   - attributeIds: Specific attributes to subscribe (empty = all)
//
// # Coalescing Behavior
//
// When multiple changes occur within minInterval, only the final value is sent.
// This reduces bandwidth while ensuring controllers see the current state.
//
// The coalescing window starts when the first change occurs after the previous
// notification. Changes are accumulated until minInterval expires.
//
// # Bounce-Back Suppression
//
// If a value changes and then returns to its original value within the
// coalescing window, no notification is sent (net change is zero).
// This prevents unnecessary traffic from temporary fluctuations.
//
// # Priming and Heartbeat
//
// When a subscription is established, a priming notification is sent
// immediately with all current values. Heartbeat notifications are sent
// at maxInterval if no changes occur, confirming the subscription is alive.
//
// # Lifecycle
//
// Subscriptions do NOT survive connection loss. On reconnect, controllers
// must re-establish subscriptions and receive new priming notifications.
package subscription
