# Connection State Machine Behavior

> Precise specification of transport layer connection lifecycle

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

MASH uses persistent TCP/TLS connections between controllers (clients) and devices (servers). This document specifies the exact connection states, transitions, and timing requirements.

**Key principles:**
- Controller always initiates connection (no race conditions)
- One connection per controller-device pair
- Connection loss triggers device FAILSAFE behavior (see state-machines.md)
- Subscriptions are bound to connections

---

## 2. Connection States

### 2.1 Server (Device) States

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Device Connection States                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────┐        TLS accept        ┌──────────────┐            │
│   │ LISTENING│ ────────────────────────►│ TLS_HANDSHAKE│            │
│   └──────────┘                          └──────┬───────┘            │
│        ▲                                       │                     │
│        │                                       ▼                     │
│        │                              ┌────────────────┐             │
│        │                              │ AUTHENTICATING │             │
│        │                              └────────┬───────┘             │
│        │                                       │                     │
│        │           ┌───────────────────────────┼───────────────┐    │
│        │           │                           ▼               │    │
│        │           │              ┌────────────────────┐       │    │
│        │           │              │   COMMISSIONING    │───────┘    │
│        │           │              │   (PASE session)   │ failure    │
│        │           │              └─────────┬──────────┘            │
│        │           │                        │ success               │
│        │           │                        ▼                       │
│        │           │              ┌────────────────────┐            │
│        │           │              │    OPERATIONAL     │◄───┐       │
│        │           │              │  (CASE session)    │    │       │
│        │           │              └─────────┬──────────┘    │       │
│        │           │                        │               │       │
│        │           │                        ▼               │       │
│        │           │              ┌────────────────────┐    │       │
│        │           │              │     CLOSING        │    │       │
│        │           │              └─────────┬──────────┘    │       │
│        │           │                        │               │       │
│        │           └────────────────────────┼───────────────┘       │
│        │                                    │ reconnect              │
│        └────────────────────────────────────┘                       │
│                          closed                                      │
└─────────────────────────────────────────────────────────────────────┘
```

| State | Description | Valid Operations |
|-------|-------------|------------------|
| LISTENING | Waiting for connections | None (TCP accept) |
| TLS_HANDSHAKE | TLS 1.3 negotiation | None |
| AUTHENTICATING | Verifying certificate or PASE | None |
| COMMISSIONING | PASE session for new zone | Commissioning messages only |
| OPERATIONAL | Normal operation with zone | All MASH operations |
| CLOSING | Graceful shutdown | Close acknowledgment only |

### 2.2 Client (Controller) States

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Controller Connection States                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────────┐      resolve       ┌──────────────┐              │
│   │ DISCONNECTED │ ─────────────────► │  CONNECTING  │              │
│   └──────────────┘                    └──────┬───────┘              │
│        ▲    ▲                                │                       │
│        │    │                                ▼                       │
│        │    │                         ┌──────────────┐              │
│        │    │                         │ TLS_HANDSHAKE│              │
│        │    │                         └──────┬───────┘              │
│        │    │                                │                       │
│        │    │      ┌─────────────────────────┼─────────────────┐    │
│        │    │      │                         ▼                 │    │
│        │    │      │              ┌──────────────────┐         │    │
│        │    │      │              │  COMMISSIONING   │─────────┘    │
│        │    │      │              │  (PASE session)  │ failure      │
│        │    │      │              └────────┬─────────┘              │
│        │    │      │                       │ success                │
│        │    │      │                       ▼                        │
│        │    │      │              ┌──────────────────┐              │
│        │    │      │              │   OPERATIONAL    │◄───┐         │
│        │    │      │              │  (CASE session)  │    │         │
│        │    │      │              └────────┬─────────┘    │         │
│        │    │      │                       │              │         │
│        │    │      │                       ▼              │         │
│        │    │      │              ┌──────────────────┐    │         │
│        │    │      │              │    CLOSING       │    │         │
│        │    │      │              └────────┬─────────┘    │         │
│        │    │      │                       │              │         │
│        │    │      └───────────────────────┼──────────────┘         │
│        │    │                              │ reconnect               │
│        │    └──────────────────────────────┘                        │
│        │                   closed                                    │
│        │                                                             │
│   ┌────┴───────┐                                                    │
│   │ RECONNECTING│ (with backoff)                                    │
│   └────────────┘                                                    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

| State | Description | Valid Operations |
|-------|-------------|------------------|
| DISCONNECTED | No connection | Connect() |
| CONNECTING | TCP connect in progress | None |
| TLS_HANDSHAKE | TLS 1.3 negotiation | None |
| COMMISSIONING | PASE session for joining zone | Commissioning messages |
| OPERATIONAL | Normal operation | All MASH operations |
| CLOSING | Graceful shutdown | Close acknowledgment |
| RECONNECTING | Waiting for backoff timer | None |

---

## 3. State Transition Rules

### 3.1 Server (Device) Transitions

```
LISTENING → TLS_HANDSHAKE      : TCP connection accepted
LISTENING → LISTENING          : [stays listening for other connections]

TLS_HANDSHAKE → AUTHENTICATING : TLS handshake completes successfully
TLS_HANDSHAKE → LISTENING      : TLS handshake fails (timeout, cipher mismatch)

AUTHENTICATING → COMMISSIONING : No operational cert presented, device in commissioning mode
AUTHENTICATING → OPERATIONAL   : Valid operational cert verified (same zone)
AUTHENTICATING → LISTENING     : Invalid cert, wrong zone, or auth timeout

COMMISSIONING → OPERATIONAL    : Commissioning completes (cert installed)
COMMISSIONING → LISTENING      : Commissioning fails or timeout

OPERATIONAL → CLOSING          : Graceful close initiated (either side)
OPERATIONAL → LISTENING        : Connection lost (TCP error, timeout)

CLOSING → LISTENING            : Close handshake complete or timeout
```

### 3.2 Client (Controller) Transitions

```
DISCONNECTED → CONNECTING      : Connect() called
DISCONNECTED → DISCONNECTED    : [idle]

CONNECTING → TLS_HANDSHAKE     : TCP connect succeeds
CONNECTING → RECONNECTING      : TCP connect fails

TLS_HANDSHAKE → COMMISSIONING  : No operational cert, starting PASE
TLS_HANDSHAKE → OPERATIONAL    : Mutual cert auth succeeds (same zone)
TLS_HANDSHAKE → RECONNECTING   : TLS handshake fails

COMMISSIONING → OPERATIONAL    : Commissioning completes
COMMISSIONING → DISCONNECTED   : Commissioning fails (user cancels, PASE fails)

OPERATIONAL → CLOSING          : Disconnect() called
OPERATIONAL → RECONNECTING     : Connection lost unexpectedly

CLOSING → DISCONNECTED         : Close handshake complete
CLOSING → RECONNECTING         : Close timeout (connection lost during close)

RECONNECTING → CONNECTING      : Backoff timer expires
RECONNECTING → DISCONNECTED    : Disconnect() called (cancel reconnection)
```

### 3.3 Invalid Transitions

Any transition not listed above is INVALID. Implementation MUST NOT allow:
- LISTENING → OPERATIONAL (must go through handshake)
- COMMISSIONING → RECONNECTING (commissioning failure is terminal)
- OPERATIONAL → COMMISSIONING (must disconnect and start fresh)

---

## 4. Timing Requirements

### 4.1 Connection Establishment

| Phase | Timeout | Rationale |
|-------|---------|-----------|
| TCP connect | 10 seconds | DNS resolution + TCP handshake |
| TLS handshake | 15 seconds | Certificate exchange + verification |
| Authentication | 10 seconds | Certificate chain validation |
| **Total establishment** | **35 seconds max** | Sum of above |

### 4.2 Commissioning Timing

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| PASE handshake | 30 seconds | SPAKE2+ computation + verification |
| CSR generation | 10 seconds | Key generation on constrained device |
| Cert installation | 5 seconds | Certificate storage |
| **Total commissioning** | **60 seconds max** | User can retry if timeout |
| Commissioning mode window | 2 minutes | From button press/QR display |

**Device commissioning mode:**
- Device enters commissioning mode via physical action (button, display)
- Mode expires after 2 minutes of no commissioning attempt
- Only one commissioning session at a time
- Existing zone connections remain operational during commissioning

### 4.3 Keep-Alive Timing

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Ping interval | 30 seconds | Balance overhead vs detection speed |
| Pong timeout | 5 seconds | Network latency allowance |
| Missed pongs before close | 3 | Avoid false positives |
| **Max detection delay** | **95 seconds** | 3 × 30s + 5s |
| **Recommended detection** | **35 seconds** | 1 × 30s + 5s (TCP keepalive helps) |

**Ping/pong rules:**
- Sender: Send ping if no message sent in last 30 seconds
- Receiver: Respond to ping immediately with pong (not subject to minInterval)
- Either side can initiate ping
- Ping during message exchange: pong still required

### 4.4 Reconnection Backoff

| Attempt | Delay | Cumulative |
|---------|-------|------------|
| 1 | 1 second | 1s |
| 2 | 2 seconds | 3s |
| 3 | 4 seconds | 7s |
| 4 | 8 seconds | 15s |
| 5 | 16 seconds | 31s |
| 6 | 32 seconds | 63s |
| 7+ | 60 seconds | +60s each |

**Backoff rules:**
- Timer starts when connection closes (not when loss detected)
- Jitter: +/- 10% randomization to avoid thundering herd
- Reset backoff to 1s after successful OPERATIONAL state (not just TCP connect)
- Max attempts: Unlimited (device may be offline indefinitely)
- Cancel backoff: Only on explicit Disconnect() call

### 4.5 Graceful Close Timing

| Parameter | Value |
|-----------|-------|
| Close acknowledgment timeout | 5 seconds |
| Outstanding request timeout | 10 seconds |
| Buffer flush timeout | 5 seconds |
| **Total close time** | **20 seconds max** |

---

## 5. Message Handling by State

### 5.1 Valid Messages per State

| State | Can Send | Can Receive |
|-------|----------|-------------|
| TLS_HANDSHAKE | TLS only | TLS only |
| AUTHENTICATING | Auth messages | Auth messages |
| COMMISSIONING | PASE, cert messages | PASE, cert messages |
| OPERATIONAL | All MASH messages | All MASH messages |
| CLOSING | Close ack only | Close ack, pending responses |

### 5.2 Message Queuing

**Outgoing queue (controller):**
- Queue messages while RECONNECTING (up to 100 messages or 1MB)
- Discard queue if reconnection fails (transition to DISCONNECTED)
- Replay queue in order after reaching OPERATIONAL
- Timeout queued messages after 60 seconds

**Incoming queue (device):**
- No queuing - messages delivered immediately or dropped
- Device does NOT buffer notifications for disconnected controllers

### 5.3 Request/Response Correlation

| Parameter | Value |
|-----------|-------|
| Request timeout | 30 seconds (configurable per operation) |
| Max in-flight requests | 10 per connection |
| Response ordering | Responses may arrive out of order |

**Correlation rules:**
- Each request has unique `messageId` (uint32, monotonically increasing per connection)
- Response includes same `messageId`
- Timeout per-request, not global
- On timeout: Mark request failed, DO NOT retry automatically (let application decide)

### 5.4 In-Flight Command Handling on Connection Loss

**Problem:** What happens to commands that were sent but not yet acknowledged when connection is lost?

**In-flight command states:**

| State | Description | Action on Connection Loss |
|-------|-------------|--------------------------|
| QUEUED | Not yet sent (in queue) | Keep in queue for replay |
| SENT_AWAITING_RESPONSE | Sent, waiting for response | Mark as UNKNOWN_OUTCOME |
| COMPLETED | Response received | No action needed |

**UNKNOWN_OUTCOME handling:**

Commands in SENT_AWAITING_RESPONSE state have uncertain outcomes:
- Command may have been processed (device acted, but response lost)
- Command may have been received but not processed (connection lost during processing)
- Command may not have been received (lost in transit)

**Command idempotency:**

| Command Type | Idempotent | Safe to Retry |
|--------------|------------|---------------|
| Read operations | Yes | Always safe |
| SetLimit | Yes | Safe (same limit value) |
| ClearLimit | Yes | Safe (idempotent) |
| SetSetpoint | Yes | Safe (same setpoint value) |
| Pause | Yes | Safe (state machine) |
| Resume | Yes | Safe (state machine) |
| Start (with parameters) | **No** | **Dangerous** - may restart process |
| Stop | Yes | Safe (state machine) |
| Subscribe | Yes | Safe (device deduplicates) |

**Controller retry policy:**

```python
def handle_in_flight_commands_after_reconnect(commands: List[Command], conn: Connection) -> None:
    """Handle commands that were in-flight when connection was lost."""

    for cmd in commands:
        if cmd.state == CommandState.QUEUED:
            # Never sent - safe to send now
            conn.send(cmd)

        elif cmd.state == CommandState.SENT_AWAITING_RESPONSE:
            if cmd.is_idempotent:
                # Safe to retry
                cmd.message_id = next_message_id()  # New ID for new connection
                conn.send(cmd)
            else:
                # Not safe to retry - notify application
                cmd.state = CommandState.UNKNOWN_OUTCOME
                notify_application(cmd, "Command outcome unknown - manual verification needed")

        # COMPLETED commands need no action
```

**Read-before-retry pattern:**

For non-idempotent commands, controller SHOULD verify state before retrying:

```python
def retry_start_command(device_id: str, process_params: dict, conn: Connection) -> bool:
    """Retry Start command safely."""

    # Read current process state first
    state = conn.read_attribute(endpoint=1, feature="EnergyControl", attr="processState")

    if state == "RUNNING":
        # Process already started - command succeeded
        return True
    elif state == "IDLE":
        # Process not started - safe to retry
        return conn.invoke("Start", process_params)
    else:
        # PAUSED, ERROR, etc. - ambiguous, ask user
        raise AmbiguousStateError(f"Process in {state} state, manual intervention needed")
```

**Notification to application:**

When command outcome is unknown, controller MUST notify application:

```python
class CommandResult:
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    UNKNOWN = "unknown"      # New state for connection loss scenarios
    RETRY_SCHEDULED = "retry_scheduled"  # Idempotent command will be retried
```

**Test cases for in-flight commands:**

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-INFLIGHT-1 | Queued command | Command in queue, disconnect | Sent after reconnect |
| TC-INFLIGHT-2 | Idempotent retry | SetLimit in flight, disconnect | Retried after reconnect |
| TC-INFLIGHT-3 | Non-idempotent | Start in flight, disconnect | Marked UNKNOWN |
| TC-INFLIGHT-4 | Read before retry | Start uncertain | State read before retry |
| TC-INFLIGHT-5 | Multiple in-flight | Several commands | Each handled per type |

---

## 6. Connection Loss Behavior

### 6.1 Detection Mechanisms

Connection loss is detected by (in order of speed):

1. **TCP layer** - FIN, RST packets (immediate)
2. **TLS layer** - Alert records (immediate)
3. **Application ping/pong** - 3 missed pongs (95 seconds max)

Device MUST use all three mechanisms. Application-layer ping/pong is required because:
- NAT/firewall may drop idle connections silently
- Half-open TCP connections may not be detected

### 6.2 Server (Device) Behavior on Connection Loss

1. Mark connection as closed
2. Cancel all pending response timers for that connection
3. Remove all subscriptions for that connection
4. If this was the only controller connection:
   - Trigger ControlState → FAILSAFE (see state-machines.md)
5. Continue listening for new/reconnecting controllers
6. Log connection loss with timestamp and reason

### 6.3 Client (Controller) Behavior on Connection Loss

1. Mark connection as closed
2. Transition to RECONNECTING
3. Start backoff timer
4. Queue new requests (up to limit)
5. Notify application of connection loss
6. Attempt reconnection when backoff expires
7. On reconnection success:
   - Re-establish all subscriptions
   - Replay queued requests
   - Notify application of restoration

### 6.4 Subscription Loss

**On connection loss:**
- All subscriptions for that connection are LOST
- Device removes subscription state
- Device stops sending notifications to that connection

**On reconnection:**
- Controller MUST re-subscribe
- Device sends priming report (full current state)
- No "missed" notifications are replayed

### 6.5 Certificate Expiry During Disconnection

**Scenario:** Controller is disconnected (reconnecting with backoff) and certificate expires during the disconnection period.

**Detection:** During TLS handshake on reconnection attempt:
- Controller presents its operational certificate
- Device checks certificate validity (notAfter)
- If expired: TLS handshake fails with `certificate_expired` alert

**Controller behavior on cert expiry detection:**

```python
def handle_reconnect_tls_failure(error: TLSError, zone: Zone) -> Action:
    """Handle TLS failure during reconnection."""

    if error.alert == "certificate_expired":
        # Check if OUR cert expired (controller) or THEIRS (device)
        if zone.controller_cert.not_after < now():
            # Our cert expired - renew locally and retry
            new_cert = zone.ca.issue_certificate(
                subject=zone.controller_cert.subject,
                public_key=zone.controller_key.public_key()
            )
            zone.controller_cert = new_cert
            return Action.RETRY_IMMEDIATELY
        else:
            # Device cert expired - cannot recover, device needs recommissioning
            log.error(f"Device {zone.device_id} certificate expired")
            return Action.MARK_DEVICE_OFFLINE

    elif error.alert == "unknown_ca":
        # Zone CA mismatch - device may have been recommissioned
        log.warning(f"Device {zone.device_id} no longer recognizes zone")
        return Action.MARK_DEVICE_REMOVED

    else:
        # Other TLS error - continue reconnection backoff
        return Action.CONTINUE_BACKOFF
```

**Device behavior on controller cert expiry:**

1. Reject TLS handshake with `certificate_expired` alert
2. Continue listening for new connections
3. Do NOT trigger FAILSAFE (controller might reconnect with renewed cert)
4. Log the rejected connection attempt

**Recovery paths:**

| Expired Cert | Recovery | Who Initiates |
|--------------|----------|---------------|
| Controller cert | Controller self-issues new cert from Zone CA | Controller |
| Device cert | Recommission device (new PASE + new cert) | Controller |
| Both certs | Recommission device | Controller |
| Zone CA cert | Cannot recover - create new zone | Controller |

**Grace period handling:**

If device supports grace period (`MASH.S.CERT.GRACE_PERIOD_DAYS > 0`):

1. Device accepts reconnection with expired cert (within grace period)
2. Device sends `cert_expiring` event immediately after connection established
3. Controller MUST initiate renewal before grace period ends
4. Connection closes if grace period expires without renewal

### 6.6 Subscription Restoration After Reconnection

**Problem:** When controller reconnects after connection loss, all subscriptions were lost. Controller needs to re-establish subscriptions efficiently.

**Controller subscription restoration workflow:**

```python
def restore_subscriptions_after_reconnect(conn: Connection, zone: Zone) -> None:
    """Restore subscriptions after successful reconnection."""

    # Step 1: Wait for connection to reach OPERATIONAL state
    assert conn.state == ConnectionState.OPERATIONAL

    # Step 2: Re-subscribe to previously subscribed attributes
    # Controller should maintain list of "desired subscriptions" locally
    for sub in zone.desired_subscriptions:
        try:
            response = conn.subscribe(
                endpoint=sub.endpoint,
                feature=sub.feature,
                attributes=sub.attributes,
                min_interval=sub.min_interval,
                max_interval=sub.max_interval
            )
            # Device sends priming report with current values
            handle_priming_report(response.priming_data)
        except SubscriptionError as e:
            log.warning(f"Failed to restore subscription: {e}")
            # Continue with other subscriptions

    # Step 3: Detect state changes that occurred during disconnection
    # Compare priming report values with last known values
    for attr, new_value in priming_data.items():
        if attr in last_known_values:
            if new_value != last_known_values[attr]:
                notify_application(attr, old=last_known_values[attr], new=new_value)

    # Step 4: Process queued commands (if any)
    replay_queued_commands(conn, zone)
```

**Subscription ordering:**

Controller SHOULD restore subscriptions in this order (most critical first):

1. **ControlState** - Know current device operating state
2. **effectiveLimits** - Know current limit values
3. **ProcessState** - For devices with processes (EV charging, etc.)
4. **Measurement** data - Power, energy, current readings
5. **Other attributes** - Status, info, etc.

**Priming report handling:**

When subscription is established, device sends a **priming report** containing:
- Current values of all subscribed attributes
- Same format as delta notifications
- Allows controller to sync state without separate reads

```cbor
// Priming report (device → controller)
{
  1: 0,                       // notification type
  2: <subscriptionId>,
  3: <endpoint>,
  4: <feature>,
  5: {                        // attribute values (full state)
    "controlState": "LIMITED",
    "effectiveConsumptionLimit": 5000000,
    "effectiveProductionLimit": null,
    "zoneLimits": [
      { "zoneId": "A1B2...", "consumptionLimit": 5000000 }
    ]
  }
}
```

**Detecting missed events:**

Events that occurred during disconnection are NOT replayed. However:

1. Controller can infer some changes by comparing priming report to last known state
2. For critical events (state changes), device SHOULD maintain recent event log
3. Controller MAY read recent events after reconnection: `ReadEvents(since=lastEventTimestamp)`

**Test cases for subscription restoration:**

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-SUB-RESTORE-1 | Basic restoration | Disconnect, reconnect | Subscriptions re-established |
| TC-SUB-RESTORE-2 | Value changed | Limit changed during disconnect | Priming report shows new value |
| TC-SUB-RESTORE-3 | Partial failure | One subscription fails | Others still restored |
| TC-SUB-RESTORE-4 | Order preserved | Multiple subscriptions | Critical subscribed first |
| TC-SUB-RESTORE-5 | Queued commands | Commands queued during disconnect | Replayed after restore |

---

## 7. Graceful Close Handshake

### 7.1 Close Message

```cbor
{
  "type": "close",       // message type
  "reason": "<string>",  // human-readable reason
  "code": <uint8>        // machine-readable code
}
```

**Close codes:**

| Code | Name | Meaning |
|------|------|---------|
| 0 | NORMAL | Normal shutdown (user-initiated) |
| 1 | GOING_AWAY | Device/controller shutting down |
| 2 | PROTOCOL_ERROR | Protocol violation detected |
| 3 | UNAUTHORIZED | Authentication/authorization failure |
| 4 | TIMEOUT | Idle timeout exceeded |
| 5 | INTERNAL_ERROR | Internal error, cannot continue |
| 6 | CERTIFICATE_EXPIRING | Certificate approaching expiry |
| 7 | ZONE_REMOVED | Zone membership revoked |

### 7.2 Close Acknowledgment

```cbor
{
  "type": "close_ack"
}
```

### 7.3 Close Sequence

**Initiator (client or server):**
1. Stop sending new requests
2. Wait for pending responses (up to 10 seconds)
3. Send close message
4. Wait for close_ack (up to 5 seconds)
5. Close TCP connection

**Receiver:**
1. On receiving close message:
2. Send any pending responses
3. Send close_ack
4. Close TCP connection

**Timeout behavior:**
- If close_ack not received within 5 seconds: Force close TCP
- If pending responses not received within 10 seconds: Send close anyway

---

## 8. Multi-Connection Handling

### 8.1 Device Connection Limits

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Max concurrent zone connections | 5 | Match max zones |
| Max concurrent commissioning | 1 | Simplify state |
| Connection per zone | 1 | No redundant connections |

### 8.2 Concurrent Connection Rules

**Multiple zones connected:**
- Each zone has independent connection
- Subscriptions are per-connection, per-zone
- Limits/setpoints resolved across all connected zones (see multi-zone-resolution.md)
- Connection loss for one zone does not affect others

**New connection from same zone:**
- If existing connection from zone is OPERATIONAL: Reject new connection
- If existing connection is stale (no activity for 60 seconds): Close old, accept new
- Controller should close old connection before opening new one

### 8.3 Resource Limits

| Resource | Limit per Connection | Limit per Device |
|----------|----------------------|------------------|
| Subscriptions | 50 | 100 |
| Pending requests | 10 | 30 |
| Message queue | 1 MB | 2 MB |
| Event buffer | 100 events | 200 events |

**Exceeding limits:**
- New subscription when at limit: Error RESOURCE_EXHAUSTED
- New request when at limit: Error BUSY (retry later)
- Message queue full: Drop oldest queued message

---

## 9. Error Recovery

### 9.1 Recoverable Errors

| Error | Recovery Action | Connection State |
|-------|----------------|------------------|
| CBOR parse error | Send error response | Keep open |
| Unknown message type | Send error response | Keep open |
| Invalid endpoint | Send error response | Keep open |
| Constraint violation | Send error response | Keep open |
| BUSY | Retry after delay | Keep open |

### 9.2 Fatal Errors

| Error | Action | Connection State |
|-------|--------|------------------|
| Invalid length prefix | Close immediately | Transition to reconnect |
| Message too large (>64KB) | Close immediately | Transition to reconnect |
| TLS alert | Close immediately | Transition to reconnect |
| Protocol version mismatch | Close with error | Transition to DISCONNECTED |
| Zone mismatch on reconnect | Close with error | Transition to DISCONNECTED |

### 9.3 Error Response Format

```cbor
{
  "messageId": <uint32>,     // From request
  "status": <uint8>,         // Error code
  "error": "<string>"        // Human-readable description
}
```

---

## 10. PICS Items

```
# Connection establishment
MASH.S.TRANS.CONN_TCP_TIMEOUT=10          # TCP connect timeout in seconds
MASH.S.TRANS.CONN_TLS_TIMEOUT=15          # TLS handshake timeout in seconds
MASH.S.TRANS.CONN_AUTH_TIMEOUT=10         # Authentication timeout in seconds

# Keep-alive
MASH.S.TRANS.PING_INTERVAL=30             # Ping interval in seconds
MASH.S.TRANS.PONG_TIMEOUT=5               # Pong timeout in seconds
MASH.S.TRANS.MISSED_PONGS_CLOSE=3         # Missed pongs before close

# Reconnection
MASH.C.TRANS.BACKOFF_INITIAL=1            # Initial backoff in seconds
MASH.C.TRANS.BACKOFF_MAX=60               # Maximum backoff in seconds
MASH.C.TRANS.BACKOFF_JITTER=10            # Jitter percentage (+/-)

# Resource limits
MASH.S.TRANS.MAX_CONNECTIONS=5            # Maximum concurrent connections
MASH.S.TRANS.MAX_SUBSCRIPTIONS=100        # Maximum total subscriptions
MASH.S.TRANS.MAX_MSG_SIZE=65536           # Maximum message size in bytes
MASH.S.TRANS.MAX_IN_FLIGHT=30             # Maximum pending requests

# Graceful close
MASH.S.TRANS.CLOSE_ACK_TIMEOUT=5          # Close ack timeout in seconds
MASH.S.TRANS.CLOSE_PENDING_TIMEOUT=10     # Pending response timeout on close

# Behavior flags
MASH.S.TRANS.B_QUEUE_ON_RECONNECT=1       # Controller queues during reconnect
MASH.S.TRANS.B_SUBSCRIPTION_LOST=1        # Subscriptions lost on disconnect
MASH.S.TRANS.B_MULTI_ZONE_CONN=1          # Supports multiple zone connections
```

---

## 11. Test Cases

### TC-CONN-*: Connection Establishment

| ID | Description | Precondition | Steps | Expected |
|----|-------------|--------------|-------|----------|
| TC-CONN-1 | Basic connection | Device listening | Controller connects | OPERATIONAL state |
| TC-CONN-2 | TCP timeout | Device unreachable | Controller connects | Timeout after 10s |
| TC-CONN-3 | TLS failure | Invalid cipher | Controller connects | Close, RECONNECTING |
| TC-CONN-4 | Auth failure | Wrong zone cert | Controller connects | Close, DISCONNECTED |
| TC-CONN-5 | Concurrent connect | Already connected | Second connect | Rejected |

### TC-KEEPALIVE-*: Keep-Alive

| ID | Description | Precondition | Steps | Expected |
|----|-------------|--------------|-------|----------|
| TC-KEEPALIVE-1 | Ping response | OPERATIONAL | Wait 30s idle | Ping sent, pong received |
| TC-KEEPALIVE-2 | Pong timeout | OPERATIONAL | Block pong | Close after 5s |
| TC-KEEPALIVE-3 | 3 missed pongs | OPERATIONAL | Block all pongs | Close after ~95s |
| TC-KEEPALIVE-4 | Activity resets | OPERATIONAL | Send message | No ping for 30s |

### TC-RECONN-*: Reconnection

| ID | Description | Precondition | Steps | Expected |
|----|-------------|--------------|-------|----------|
| TC-RECONN-1 | Basic reconnect | OPERATIONAL | Kill TCP | Reconnect with backoff |
| TC-RECONN-2 | Backoff timing | Connection lost | Observe attempts | 1s, 2s, 4s, 8s... |
| TC-RECONN-3 | Backoff reset | Reconnected | Lose again | Starts at 1s |
| TC-RECONN-4 | Cancel reconnect | RECONNECTING | Call Disconnect() | DISCONNECTED |

### TC-CLOSE-*: Graceful Close

| ID | Description | Precondition | Steps | Expected |
|----|-------------|--------------|-------|----------|
| TC-CLOSE-1 | Clean close | OPERATIONAL | Send close | close_ack, TCP closed |
| TC-CLOSE-2 | Pending requests | Request in flight | Send close | Response, then close |
| TC-CLOSE-3 | Close timeout | OPERATIONAL | Send close, no ack | TCP force close after 5s |
| TC-CLOSE-4 | Simultaneous close | Both sides close | Send close | Both ack, both close |

### TC-MULTI-*: Multi-Connection

| ID | Description | Precondition | Steps | Expected |
|----|-------------|--------------|-------|----------|
| TC-MULTI-1 | Two zones | Device listening | Two controllers connect | Both OPERATIONAL |
| TC-MULTI-2 | Zone limit | 2 zones connected | 6th connects | Rejected |
| TC-MULTI-3 | Zone isolation | Two zones connected | Zone A subscribes | Zone B sees nothing |
| TC-MULTI-4 | Partial loss | Two zones connected | Zone A disconnects | Zone B unaffected |

---

## 12. Implementation Notes

### 12.1 Recommended TCP Options

```
TCP_NODELAY = 1           # Disable Nagle for low latency
SO_KEEPALIVE = 1          # Enable TCP keepalive
TCP_KEEPIDLE = 30         # Start probes after 30s idle
TCP_KEEPINTVL = 10        # Probe interval
TCP_KEEPCNT = 3           # Probes before failure
```

### 12.2 TLS Configuration

- TLS 1.3 only (no fallback)
- Cipher suites: See tls-profile.md section 3
- ALPN: "mash/1" (protocol identifier)
- SNI: Device ID or IP (for certificate selection)

### 12.3 Thread Safety

- Connection state machine MUST be thread-safe
- State transitions MUST be atomic
- Message send/receive MUST be serialized per connection
- Multiple connections MUST be independent
