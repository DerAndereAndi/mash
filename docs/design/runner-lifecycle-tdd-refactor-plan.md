# Runner Lifecycle Determinism: TDD Refactor Plan

**Date:** 2026-02-17
**Scope:** `mash-go/internal/testharness/runner`
**Goal:** Replace best-effort cleanup/fallback orchestration with deterministic lifecycle transitions and verifiable cleanup invariants.

---

## Problem Statement

Current failures are driven by three structural issues:

1. Cleanup paths are best-effort and often continue after failure.
2. Multiple mutable connection authorities (`pool.Main`, `suite.Conn`, tracked zone conns, crypto snapshots) create ambiguous ownership.
3. Fallback routing hides which connection actually performed state-changing operations.

This plan introduces a strict lifecycle path behind a feature flag, proves behavior with tests first, then flips Phase 1 runs to strict mode.

---

## Non-Goals

- No rewrite of protocol action handlers.
- No change to YAML test format.
- No immediate deletion of legacy paths (kept behind compatibility flag during migration).

---

## Architecture Target

Introduce a deterministic lifecycle contract:

- Single authoritative control channel per phase.
- Explicit state transitions only (`Disconnected -> CommissioningTLS -> OperationalTLS`).
- Cleanup returns a structured report with invariant checks.
- State-changing actions require explicit connection target (no implicit fallback).

New flag in runner config:

- `StrictLifecycle bool` (default `false` initially)

When `StrictLifecycle=true`:

- Cleanup failures are terminal for the suite run.
- `RemoveZone` failures are surfaced as hard errors.
- Reconnect failures in teardown are hard errors.
- Invariant report is emitted after each test.

---

## Phase 1: Cleanup Invariants (Foundation)

### Red tests first

Add `mash-go/internal/testharness/runner/cleanup_invariants_test.go`:

1. `TestCleanupReport_DetectsPhantomMainSocket`
2. `TestCleanupReport_DetectsPhantomZoneSocket`
3. `TestCleanupReport_DetectsNonEmptyZonePool`
4. `TestCleanupReport_DetectsIncompletePASE`
5. `TestCleanupReport_DetectsResidualSuiteConnection`
6. `TestCleanupReport_AllClean`

### Implementation

Add `mash-go/internal/testharness/runner/cleanup_invariants.go`:

- `type CleanupReport struct { ... }`
- `func (r *Runner) BuildCleanupReport() CleanupReport`
- `func (cr CleanupReport) IsClean() bool`

Integrate into teardown:

- `mash-go/internal/testharness/runner/coordinator.go` (`TeardownTest`)
- In strict mode: fail fast if report is not clean.
- In legacy mode: log report only.

### Done when

- New invariant tests pass.
- Existing runner tests pass.

---

## Phase 2: Strict RemoveZone Semantics

### Red tests first

Extend `mash-go/internal/testharness/runner/preconditions_test.go`:

1. `TestSendRemoveZone_StrictMode_FailsWhenNoLiveConn`
2. `TestSendRemoveZone_StrictMode_FailsWhenNoPASEState`
3. `TestSendRemoveZone_StrictMode_FailsOnWriteError`
4. `TestSendRemoveZone_StrictMode_SucceedsOnAck`

Extend `mash-go/internal/testharness/runner/coordinator_test.go`:

5. `TestCoordFreshCommission_StrictMode_BubblesRemoveZoneFailure`

### Implementation

Refactor in `mash-go/internal/testharness/runner/preconditions.go`:

- Keep legacy `sendRemoveZone()` behavior for compatibility.
- Add strict variant returning error:
  - `func (r *Runner) sendRemoveZoneStrict() error`
  - `func (r *Runner) sendRemoveZoneOnConnStrict(conn *Connection, zoneID string) error`

Use strict functions when `config.StrictLifecycle` is true.

### Done when

- Strict mode no longer silently skips RemoveZone.
- Tests prove failure propagation.

---

## Phase 3: Teardown Error Policy

### Red tests first

Extend `mash-go/internal/testharness/runner/coordinator_test.go`:

1. `TestCoordTeardown_StrictMode_ReconnectFailureIsFatal`
2. `TestCoordTeardown_StrictMode_ResetRetryFailureIsFatal`
3. `TestCoordTeardown_LegacyMode_ReconnectFailureContinues`

### Implementation

Refactor `mash-go/internal/testharness/runner/coordinator.go`:

- Replace `(continuing)` branches with:
  - strict mode: return typed teardown error
  - legacy mode: keep current behavior

Add error type in `mash-go/internal/testharness/runner/errors.go`:

- `type TeardownError struct { Step string; Cause error }`

Wire teardown error into result output (JSON/text) so failures are visible and actionable.

### Done when

- Strict mode aborts on teardown failures.
- Legacy mode remains backward compatible.

---

## Phase 4: Explicit Connection Targeting for State-Changing Ops

### Red tests first

Add tests in:

- `mash-go/internal/testharness/runner/trigger_handlers_test.go`
- `mash-go/internal/testharness/runner/controller_handlers_test.go`

Cases:

1. `TestSendTrigger_StrictMode_RequiresExplicitConnKey`
2. `TestSendTrigger_StrictMode_RejectsFallbackToMain`
3. `TestHandleRemoveDevice_StrictMode_RequiresResolvableZoneConn`
4. `TestLegacyMode_AllowsFallback`

### Implementation

- Introduce helper in `mash-go/internal/testharness/runner/connection_handlers.go`:
  - `resolveRequiredConn(params, state) (*Connection, string, error)`
- Use helper for state-changing operations (`trigger`, `remove_zone`, controller cleanup actions).
- Keep fallback only when `StrictLifecycle=false`.

### Done when

- Strict mode never mutates device state via implicit fallback paths.

---

## Phase 5: Single Authority Lifecycle Controller

### Red tests first

Create `mash-go/internal/testharness/runner/lifecycle_controller_test.go`:

1. `TestLifecycle_Transition_DisconnectedToCommissioning`
2. `TestLifecycle_Transition_CommissioningToOperational`
3. `TestLifecycle_Transition_OperationalToDisconnected`
4. `TestLifecycle_InvalidTransitionRejected`
5. `TestLifecycle_ControlChannelAuthority_IsUnique`

### Implementation

Add `mash-go/internal/testharness/runner/lifecycle_controller.go`:

- `type LifecycleState int`
- `type LifecycleController struct { ... }`
- Explicit transition APIs used by coordinator/commissioning.

Minimize direct `pool.Main` mutations outside controller in strict mode.

### Done when

- Strict mode paths use controller transitions only.
- Transition logic is unit-tested and deterministic.

---

## Phase 6: Device Reset Contract Verification

### Red tests first

Add integration tests in `mash-go/internal/testharness/runner/commissioning_test.go`:

1. `TestStrictCleanup_LeavesDeviceCommissionable`
2. `TestStrictCleanup_NoResidualZonesAfterGroupRun`
3. `TestStrictCleanup_NoUnknownAuthorityCascadeAfterRecommission`

### Implementation

- Add post-test probe sequence in strict mode:
  - `verify_commissioning_state`
  - zone count check
  - reconnect probe
- Fail immediately if contract not met.

### Done when

- Phase 1 group runs stable in strict mode without manual resets.

---

## Phase 7: Rollout and Legacy Path Deletion

### Rollout steps

1. Land phases 1-3 with `StrictLifecycle=false` default.
2. Run CI matrix with both modes.
3. Enable strict mode for Phase 1 stabilization runs.
4. After two stable cycles, flip default to `StrictLifecycle=true`.
5. Remove legacy fallback branches in a dedicated cleanup PR.

### Exit criteria

- No `(continuing)` cleanup paths left in strict mode.
- No silent no-op RemoveZone path in strict mode.
- Phase 1 acceptance run (5 sequential + 5 shuffled) passes in strict mode.

---

## PR Slice Plan

1. `PR-1`: Cleanup report + invariant tests.
2. `PR-2`: Strict RemoveZone semantics.
3. `PR-3`: Teardown error policy + typed errors.
4. `PR-4`: Explicit connection targeting in state-mutating handlers.
5. `PR-5`: Lifecycle controller introduction.
6. `PR-6`: Strict reset contract integration tests.
7. `PR-7`: Flip default + remove legacy branches.

Each PR must include:

- Red test commit (failing tests).
- Green implementation commit.
- Optional refactor commit.

---

## Suggested Test Commands

From `mash-go/`:

```bash
# fast unit scope during development
go test ./internal/testharness/runner -run 'CleanupReport|RemoveZone|CoordTeardown|StrictMode|Lifecycle' -count=1

# full runner package
go test ./internal/testharness/runner -count=1

# targeted integration signal
go test ./internal/testharness/runner -run 'Commissioning|Reconnect|Teardown' -count=1
```

---

## Risk Controls

- Keep strict behavior behind flag until stable.
- Add JSON output field `cleanup_report` for observability.
- Do not modify device protocol semantics in this refactor.
- Keep PRs small; one behavior change domain per PR.
