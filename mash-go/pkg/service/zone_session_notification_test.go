package service

import (
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// createTestDeviceWithMeasurement creates a device with DeviceInfo and Measurement
// features for subscription notification testing.
func createTestDeviceWithMeasurement() *model.Device {
	device := model.NewDevice("test-device", 0x1234, 0x5678)

	// DeviceInfo on root endpoint
	deviceInfo := features.NewDeviceInfo()
	_ = deviceInfo.SetDeviceID("test-device")
	_ = deviceInfo.SetVendorName("Test Vendor")
	device.RootEndpoint().AddFeature(deviceInfo.Feature)

	// EVSE endpoint with Measurement feature
	evse := model.NewEndpoint(1, model.EndpointEVCharger, "Test EVSE")
	measurement := features.NewMeasurement()
	evse.AddFeature(measurement.Feature)
	_ = device.AddEndpoint(evse)

	return device
}

// extractSubIDFromPayload extracts the subscription ID from a subscribe response
// payload, handling both typed and CBOR-decoded raw map forms.
func extractSubIDFromPayload(payload any) (uint32, bool) {
	switch p := payload.(type) {
	case *wire.SubscribeResponsePayload:
		return p.SubscriptionID, true
	case map[any]any:
		if id, ok := wire.ToUint32(p[uint64(1)]); ok {
			return id, true
		}
	case map[uint64]any:
		if id, ok := wire.ToUint32(p[1]); ok {
			return id, true
		}
	}
	return 0, false
}

// TestZoneSession_SubscribeDispatchesHeartbeat verifies that subscribing via ZoneSession
// with a short maxInterval causes heartbeat notifications to be sent over the connection.
func TestZoneSession_SubscribeDispatchesHeartbeat(t *testing.T) {
	device := createTestDeviceWithMeasurement()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)
	defer session.Close()

	// Subscribe to Measurement (endpoint 1) with short maxInterval for heartbeat
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureMeasurement),
		Payload: &wire.SubscribePayload{
			MinInterval: 10,  // 10ms
			MaxInterval: 200, // 200ms - heartbeat should fire within this
		},
	}

	reqData, err := wire.EncodeRequest(req)
	if err != nil {
		t.Fatalf("encode subscribe request: %v", err)
	}
	session.OnMessage(reqData)

	// Wait briefly for the subscribe response
	time.Sleep(50 * time.Millisecond)

	// The first sent message should be the subscribe response
	sent := conn.SentMessages()
	if len(sent) < 1 {
		t.Fatalf("expected at least 1 message (subscribe response), got %d", len(sent))
	}

	resp, err := wire.DecodeResponse(sent[0])
	if err != nil {
		t.Fatalf("decode subscribe response: %v", err)
	}
	if resp.Status != wire.StatusSuccess {
		t.Fatalf("subscribe failed with status %v", resp.Status)
	}

	// Now wait for heartbeat notification to arrive (maxInterval + processing margin)
	time.Sleep(400 * time.Millisecond)

	sent = conn.SentMessages()
	// Expect: 1 subscribe response + at least 1 heartbeat notification
	if len(sent) < 2 {
		t.Fatalf("expected at least 2 messages (response + heartbeat), got %d", len(sent))
	}

	// Decode the second message as a notification
	notif, err := wire.DecodeNotification(sent[1])
	if err != nil {
		t.Fatalf("decode heartbeat notification: %v", err)
	}

	if notif.EndpointID != 1 {
		t.Errorf("heartbeat EndpointID: got %d, want 1", notif.EndpointID)
	}
	if notif.FeatureID != uint8(model.FeatureMeasurement) {
		t.Errorf("heartbeat FeatureID: got %d, want %d", notif.FeatureID, model.FeatureMeasurement)
	}
}

// TestZoneSession_SubscribeCoalesces verifies that rapid attribute changes within
// minInterval are coalesced into a single notification.
func TestZoneSession_SubscribeCoalesces(t *testing.T) {
	device := createTestDeviceWithMeasurement()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)
	defer session.Close()

	// Subscribe with minInterval=200ms to allow coalescing
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureMeasurement),
		Payload: &wire.SubscribePayload{
			MinInterval: 200,   // 200ms coalescing window
			MaxInterval: 60000, // long heartbeat so it won't interfere
		},
	}

	reqData, err := wire.EncodeRequest(req)
	if err != nil {
		t.Fatalf("encode subscribe request: %v", err)
	}
	session.OnMessage(reqData)
	time.Sleep(50 * time.Millisecond)

	// Verify subscribe succeeded
	sent := conn.SentMessages()
	if len(sent) < 1 {
		t.Fatalf("expected subscribe response, got %d messages", len(sent))
	}
	resp, _ := wire.DecodeResponse(sent[0])
	if resp.Status != wire.StatusSuccess {
		t.Fatalf("subscribe failed: status %v", resp.Status)
	}

	// The session should have a dispatcher that we can notify.
	// Trigger 3 rapid attribute changes (10ms apart) -- should coalesce.
	if session.dispatcher == nil {
		t.Fatal("expected session to have a dispatcher after subscribe")
	}

	for i := range 3 {
		session.dispatcher.NotifyChange(1, uint16(model.FeatureMeasurement), 1, int64(1000*(i+1)))
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the coalescing window + processing to complete
	time.Sleep(400 * time.Millisecond)

	sent = conn.SentMessages()
	// Count notifications (messages after the subscribe response)
	notifCount := 0
	for _, data := range sent[1:] {
		if _, err := wire.DecodeNotification(data); err == nil {
			notifCount++
		}
	}

	// With coalescing, we should get exactly 1 notification (not 3)
	if notifCount != 1 {
		t.Errorf("expected 1 coalesced notification, got %d", notifCount)
	}
}

// TestZoneSession_MultipleSubscriptionsIndependent verifies that two subscriptions
// to the same feature each produce their own notification when a change occurs.
func TestZoneSession_MultipleSubscriptionsIndependent(t *testing.T) {
	device := createTestDeviceWithMeasurement()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)
	defer session.Close()

	// Subscribe twice to Measurement (different subscription IDs)
	var subIDs []uint32
	for i := uint32(1); i <= 2; i++ {
		req := &wire.Request{
			MessageID:  i,
			Operation:  wire.OpSubscribe,
			EndpointID: 1,
			FeatureID:  uint8(model.FeatureMeasurement),
			Payload: &wire.SubscribePayload{
				MinInterval: 10,    // short
				MaxInterval: 60000, // long heartbeat
			},
		}
		reqData, err := wire.EncodeRequest(req)
		if err != nil {
			t.Fatalf("encode subscribe request %d: %v", i, err)
		}
		session.OnMessage(reqData)
	}

	time.Sleep(50 * time.Millisecond)

	// Extract subscription IDs from responses
	sent := conn.SentMessages()
	if len(sent) < 2 {
		t.Fatalf("expected 2 subscribe responses, got %d messages", len(sent))
	}

	for _, data := range sent[:2] {
		resp, err := wire.DecodeResponse(data)
		if err != nil {
			t.Fatalf("decode subscribe response: %v", err)
		}
		if resp.Status != wire.StatusSuccess {
			t.Fatalf("subscribe failed: status %v", resp.Status)
		}
		if id, ok := extractSubIDFromPayload(resp.Payload); ok {
			subIDs = append(subIDs, id)
		}
	}

	if len(subIDs) != 2 {
		t.Fatalf("expected 2 subscription IDs, got %d", len(subIDs))
	}
	if subIDs[0] == subIDs[1] {
		t.Fatalf("subscription IDs should be different, both are %d", subIDs[0])
	}

	// Trigger 1 attribute change
	if session.dispatcher == nil {
		t.Fatal("expected session to have a dispatcher")
	}
	session.dispatcher.NotifyChange(1, uint16(model.FeatureMeasurement), 1, int64(5000))

	// Wait for notifications
	time.Sleep(300 * time.Millisecond)

	sent = conn.SentMessages()
	// Collect notification subscription IDs
	notifSubIDs := make(map[uint32]bool)
	for _, data := range sent[2:] { // skip the 2 subscribe responses
		notif, err := wire.DecodeNotification(data)
		if err == nil {
			notifSubIDs[notif.SubscriptionID] = true
		}
	}

	// Both subscriptions should have received a notification
	if len(notifSubIDs) != 2 {
		t.Errorf("expected notifications for 2 subscriptions, got %d (IDs: %v)", len(notifSubIDs), notifSubIDs)
	}
	for _, id := range subIDs {
		if !notifSubIDs[id] {
			t.Errorf("subscription %d did not receive a notification", id)
		}
	}
}
