# Protocol Behaviors

> Implementation behaviors for MASH protocol operations

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

This document specifies the runtime behaviors implemented in the MASH Go reference implementation for:
- Request/response correlation
- Subscription management and priming
- Keep-alive ping/pong
- Reconnection with exponential backoff

**Reference implementations:**
- `pkg/interaction/client.go` - Request/response correlation
- `pkg/subscription/` - Subscription management
- `pkg/transport/keepalive.go` - Keep-alive
- `pkg/connection/` - Reconnection logic

---

## 2. Request/Response Correlation

### 2.1 Message ID Protocol

Each request is assigned a unique 32-bit `MessageID`:
- Starts from 1 (0 is reserved for notifications)
- Auto-incremented atomically per connection
- Responses use the same MessageID as the request

### 2.2 Pending Request Tracking

```go
type Client struct {
    pending   map[uint32]chan *wire.Response
    nextMsgID uint32
}
```

### 2.3 Request Flow

1. **Send Request:**
   - Generate unique messageID: `atomic.AddUint32(&c.nextMsgID, 1)`
   - Create response channel: `make(chan *wire.Response, 1)`
   - Store in pending map: `c.pending[req.MessageID] = respCh`
   - Encode and send request
   - Wait on channel with timeout

2. **Receive Response:**
   - Look up messageID in pending map
   - Send response to corresponding channel
   - If no pending request: return `ErrUnexpectedReply`

3. **Cleanup:**
   - Remove from pending map (defer in request handler)
   - Channel garbage collected

### 2.4 Timeout Handling

| Parameter | Default | Description |
|-----------|---------|-------------|
| Request timeout | 30s | Maximum wait for response |

On timeout:
- Pending entry removed
- `context.DeadlineExceeded` returned to caller
- Request may still complete server-side

### 2.5 Thread Safety

- `sync.RWMutex` protects pending map
- Atomic operations for message ID generation
- Buffered channel (size 1) prevents goroutine leaks

---

## 3. Subscription Management

### 3.1 Two-Level Architecture

**Device-side (Manager):**
- Tracks all active subscriptions
- Dispatches changes to affected subscriptions
- Handles priming, coalescing, heartbeats

**Client-side:**
- Sends Subscribe/Unsubscribe requests
- Receives priming report and notifications
- Routes notifications to registered handlers

### 3.2 Subscription Limits

| Parameter | Default | Description |
|-----------|---------|-------------|
| Max subscriptions | 50 | Per device |
| Max attributes | 100 | Per subscription |

### 3.3 Priming Mechanism

When a subscription is created:

1. Client sends `SubscribeRequest` with `minInterval`, `maxInterval`, attribute list
2. Device creates subscription with unique `SubscriptionID`
3. Device immediately sends priming notification with current values
4. Priming notification has `IsPriming: true` flag
5. Client receives `SubscribeResponse` with `SubscriptionID` and initial values

```go
// Priming notification (device-side)
notification := &Notification{
    SubscriptionID: sub.ID,
    IsPriming:      true,
    Changes:        currentValues,
}
```

### 3.4 Change Coalescing

Changes are coalesced based on `minInterval`:

```
T=0:    Value changes to A
T=0.5:  Value changes to B
T=1.0:  Notification sent with value B (coalesced)
```

**Rules:**
- Changes accumulated in `pendingChanges` map
- Notification sent only after `minInterval` elapses since last notification
- Multiple changes to same attribute: latest value wins

### 3.5 Heartbeat Behavior

| Parameter | Default | Description |
|-----------|---------|-------------|
| minInterval | 1s | Minimum time between notifications |
| maxInterval | 60s | Maximum time without notification (heartbeat) |

**Heartbeat modes:**
- **Empty**: Send notification with no changes (just proves liveness)
- **Full**: Re-send all current values

If no changes occur for `maxInterval`, heartbeat is sent.

### 3.6 Bounce-Back Suppression

Prevents re-notifying unchanged values:

```go
if lastVal, exists := s.lastValues[attrID]; exists {
    if valuesEqual(lastVal, newValue) {
        continue  // Skip unchanged values
    }
}
```

Enabled by default. Useful when device writes same value repeatedly.

### 3.7 Feature Index

Efficient dispatch via pre-built index:

```go
featureIndex map[featureKey][]*Subscription
```

When attribute changes:
1. Look up subscriptions by (endpointID, featureID)
2. Check if specific attribute is in subscription
3. Record change in affected subscriptions

O(1) lookup for change dispatch.

---

## 4. Keep-Alive Ping/Pong

### 4.1 Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| Ping interval | 30s | Time between pings |
| Pong timeout | 5s | Max time to wait for pong |
| Max missed pongs | 3 | Failures before disconnect |
| Max detection delay | 95s | Worst-case failure detection time |

**Detection delay calculation:**
```
max_delay = ping_interval * max_missed_pongs + pong_timeout
          = 30s * 3 + 5s = 95s
```

### 4.2 Ping/Pong Protocol

**Ping message:**
```cbor
{
    "type": "ping",
    "seq": <uint32>
}
```

**Pong message:**
```cbor
{
    "type": "pong",
    "seq": <uint32>  // Same as ping
}
```

### 4.3 State Machine

```
IDLE ──[interval elapsed]──> SEND_PING
SEND_PING ──[ping sent]──> WAITING_PONG
WAITING_PONG ──[pong received, seq matches]──> IDLE (reset missedPongs)
WAITING_PONG ──[timeout, < max missed]──> IDLE (missedPongs++)
WAITING_PONG ──[timeout, >= max missed]──> TIMEOUT (trigger disconnect)
```

### 4.4 Latency Tracking

When pong received:
- Calculate: `latency = pong_time - ping_time`
- Optional callback for telemetry/monitoring

### 4.5 Auto-Response

Device automatically responds to pings:
```go
// On receiving ControlPing
SendControlMessage(ControlPong, seq)
```

No application involvement required.

---

## 5. Reconnection with Backoff

### 5.1 Exponential Backoff

| Attempt | Delay |
|---------|-------|
| 1 | 1s |
| 2 | 2s |
| 3 | 4s |
| 4 | 8s |
| 5 | 16s |
| 6 | 32s |
| 7+ | 60s (max) |

### 5.2 Jitter

Prevents thundering herd:
```
actual_delay = base_delay + random(0, base_delay * 0.25)
```

**Example:** Base delay 8s, jitter 0-2s, actual 8-10s.

### 5.3 Backoff API

| Method | Description |
|--------|-------------|
| Next() | Return jittered delay, advance level |
| Peek() | Return jittered delay without advancing |
| Reset() | Reset to initial (1s) on success |
| Attempts() | Return attempt count since reset |

### 5.4 Connection Manager States

```
StateDisconnected  ─[connect()]──> StateConnecting
StateConnecting    ─[success]────> StateConnected
StateConnecting    ─[failure]────> StateDisconnected (+ retry if autoReconnect)
StateConnected     ─[conn lost]──> StateReconnecting
StateReconnecting  ─[success]────> StateConnected
StateReconnecting  ─[closed]─────> StateClosed
```

### 5.5 Reconnection Flow

1. **Trigger:** Connection lost (keep-alive timeout, I/O error, peer close)
2. **Calculate delay:** `backoff.Next()`
3. **Callback:** `onReconnecting(attempts, delay)`
4. **Wait:** Sleep for delay (with cancellation support)
5. **Attempt:** Call `connectFn(ctx)` with 30s timeout
6. **On success:** Reset backoff, transition to Connected
7. **On failure:** Loop to step 2

### 5.6 Backoff Reset Rules

| Event | Action |
|-------|--------|
| Successful connection (TLS complete) | Reset to 1s |
| Application-layer rejection | Do NOT reset |
| TCP/TLS failure | Do NOT reset |

**Rationale:** Application rejections (wrong credentials, certificate issues) shouldn't get fast retry. Only fully successful connections reset backoff.

### 5.7 Callbacks

| Callback | When |
|----------|------|
| OnStateChange(old, new) | State transitions |
| OnConnected() | After successful connection |
| OnDisconnected() | When connection is lost |
| OnReconnecting(attempt, delay) | Before each reconnect attempt |

---

## 6. Integration

### 6.1 Message Type Routing

Incoming messages are routed by type:

| Type | Handling |
|------|----------|
| Request | Process via ProtocolHandler, send response |
| Response | Deliver to Client via correlation |
| Notification | Deliver to subscription handler |
| Control (ping/pong/close) | Handle in transport layer |

### 6.2 Connection Loss Handling

When connection is lost:
1. Keep-alive detects (3 missed pongs)
2. Connection manager notified
3. All pending requests cancelled (timeout)
4. Subscriptions marked stale
5. Reconnection initiated with backoff
6. On reconnect: subscriptions must be recreated

### 6.3 Subscription Recovery

After reconnection:
- Previous subscriptions are NOT automatically restored
- Client must re-subscribe
- Priming notification ensures no missed state

---

## 7. PICS Items

```
# Request/Response
MASH.S.PROTO.CORRELATION         # Message ID correlation
MASH.S.PROTO.REQUEST_TIMEOUT     # 30s request timeout

# Subscriptions
MASH.S.PROTO.SUBSCRIPTION        # Subscription support
MASH.S.PROTO.PRIMING             # Priming notification on subscribe
MASH.S.PROTO.COALESCING          # Change coalescing
MASH.S.PROTO.HEARTBEAT           # Heartbeat notifications

# Keep-Alive
MASH.S.PROTO.KEEPALIVE           # Ping/pong keep-alive
MASH.S.PROTO.AUTO_PONG           # Automatic pong response

# Reconnection
MASH.C.PROTO.RECONNECT           # Auto-reconnection
MASH.C.PROTO.BACKOFF             # Exponential backoff with jitter
```

---

## 8. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-PROTO-001 | Request/response correlation | Send request, receive response | MessageIDs match |
| TC-PROTO-002 | Request timeout | Send request, no response | Timeout after 30s |
| TC-PROTO-003 | Concurrent requests | Send multiple requests | Each gets correct response |
| TC-PROTO-004 | Subscription priming | Subscribe | Receive priming notification |
| TC-PROTO-005 | Change coalescing | Rapid changes | Single notification with latest |
| TC-PROTO-006 | Heartbeat delivery | Wait maxInterval | Heartbeat sent |
| TC-PROTO-007 | Ping/pong exchange | Wait for ping interval | Pong received |
| TC-PROTO-008 | Keep-alive timeout | Block pong responses | Disconnect after 3 misses |
| TC-PROTO-009 | Reconnect with backoff | Disconnect, reconnect | Delays increase exponentially |
| TC-PROTO-010 | Backoff reset on success | Connect successfully | Backoff resets to 1s |
| TC-PROTO-011 | Jitter prevents thundering herd | Multiple reconnects | Delays differ |
| TC-PROTO-012 | Subscription recovery | Reconnect, re-subscribe | New priming received |
