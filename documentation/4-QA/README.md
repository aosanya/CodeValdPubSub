# 4 — QA

## Overview

This section covers testing strategy, acceptance criteria, and quality assurance for CodeValdPubSub.

---

## Testing Standards

All contributions must satisfy:

| Check | Command | Requirement |
|---|---|---|
| Build | `go build ./...` | Must succeed — no compilation errors |
| Unit tests | `go test -v -race ./...` | All tests green; no data races |
| Static analysis | `go vet ./...` | 0 issues |
| Linting | `golangci-lint run ./...` | Must pass |
| Coverage | `go test -coverprofile=coverage.out ./...` | Target ≥ 80% on exported functions |

---

## Test Structure Convention

Tests live alongside source files using Go's standard `_test.go` convention:

```
pubsub_test.go              ← PubSubManager lifecycle tests
internal/
  router/
    router_test.go          ← Topic pattern matching tests (no ArangoDB needed)
  server/
    server_test.go          ← gRPC handler tests
```

Integration tests that require external services (ArangoDB) must use `t.Skip()` when `ARANGODB_URL` is not set.

---

## Key Test Scenarios

### Topic Pattern Matching (unit, no ArangoDB)
- `*` matches exactly one segment
- `#` matches remaining suffix
- `#` as only token matches any topic
- Pattern with trailing `#` does not match topics shorter than the fixed prefix
- Subscription index correctly returns all matching subscriptions for a given topic

### Publish (integration)
- Valid event is written to `PubSubEvent` collection before returning
- Idempotent publish with same `IdempotencyKey` returns the existing event without creating a duplicate
- Topic with fewer than 3 segments returns `ErrTopicInvalid`
- Fan-out: publishing to a topic with 3 matching subscriptions creates 3 `PubSubDelivery` records

### Subscribe (integration)
- Creating a subscription writes to `PubSubSubscription` collection
- `DeleteSubscription` sets status to `cancelled`; subsequent publishes do not create deliveries for it
- Invalid pattern (`#` not in suffix position) returns an error

### Delivery Loop (integration)
- Pending delivery is retried after timeout
- Successful delivery advances status to `delivered`
- `Ack` advances status to `acked` and stops retries
- After max attempts, delivery status becomes `failed`

### Query / Replay (integration)
- Events returned in ascending timestamp order
- Time range filter excludes events outside the window
- Pattern filter correctly narrows results
- Pagination cursor returns the correct next page

---

## Acceptance Criteria per Task

See the `### Tests` section of each task file in [../3-SofwareDevelopment/mvp-details/](../3-SofwareDevelopment/mvp-details/) for the full test matrix per MVP task.
