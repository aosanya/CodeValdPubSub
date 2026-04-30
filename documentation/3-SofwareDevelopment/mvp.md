# MVP — Active Task Backlog

## Overview
- **Objective**: Deliver CodeValdPubSub as a production-ready durable pub/sub event recorder and routing layer for the CodeVald platform.
- **Completed tasks**: see [`mvp_done.md`](mvp_done.md)
- **Detailed specs**: see [`mvp-details/`](mvp-details/)

---

## Execution Order

```
PUBSUB-001 (domain model + schema)
    │
    ▼
PUBSUB-002 (Publish API)
    │
    ▼
PUBSUB-003 (Subscribe API)
    │
    ├──► PUBSUB-004 (Query / Replay API)   — parallel
    └──► PUBSUB-005 (Delivery loop + ACK)  — parallel
              │
              ▼
         PUBSUB-006 (gRPC proto + server)
              │
              ▼
         PUBSUB-007 (CodeValdCross integration)  ← last
```

---

## PUBSUB-001 — Domain Model & Entity Schema

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-001: Define `Event`, `Subscription`, `DeliveryRecord` value types in `models.go`; define `DefaultPubSubSchema()` in `schema.go`; register entity types (`PubSubEvent`, `PubSubSubscription`, `PubSubDelivery`) | 📋 Not Started | — |

**Scope**: Replace CodeValdGit's domain types with PubSub domain types.

- `models.go` — `Event`, `Subscription`, `DeliveryRecord`, `TopicPattern` value types
- `types.go` — `PublishRequest`, `SubscribeRequest`, `QueryEventsRequest`, request/response types
- `schema.go` — `DefaultPubSubSchema()` returning three `TypeDefinition` entries for `PubSubEvent`, `PubSubSubscription`, `PubSubDelivery`; each field as a typed `PropertyDefinition`
- `errors.go` — `ErrTopicInvalid`, `ErrSubscriptionNotFound`, `ErrDuplicateEvent`
- `pubsub.go` — `PubSubManager` interface (Publish, Subscribe, ListSubscriptions, GetSubscription, DeleteSubscription, Ack, QueryEvents) + `NewPubSubManager` constructor

See: [mvp-details/domain-model.md](mvp-details/domain-model.md)

---

## PUBSUB-002 — Publish API

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-002: Implement `Publish(ctx, PublishRequest) (Event, error)` — topic validation, idempotency check, durable write to `PubSubEvent`, fan-out routing | 📋 Not Started | PUBSUB-001 |

**Scope**:
- Validate topic format (`<service>.<agencyID>.<segments…>.<action>`, 3–10 segments, ≤ 512 chars)
- Check idempotency key: if (agencyID, topic, IdempotencyKey) exists within 24 h, return existing event
- Write `PubSubEvent` entity to ArangoDB via `entitygraph.DataManager`
- Look up all matching `PubSubSubscription` patterns from the subscription index
- For each match: create `PubSubDelivery` record (status: `pending`) and enqueue for delivery

See: [mvp-details/publish.md](mvp-details/publish.md)

---

## PUBSUB-003 — Subscribe API

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-003: Implement `Subscribe`, `ListSubscriptions`, `GetSubscription`, `DeleteSubscription` | 📋 Not Started | PUBSUB-001 |

**Scope**:
- Validate pattern syntax (`*` per segment, `#` suffix only)
- Write `PubSubSubscription` entity to ArangoDB
- Maintain an in-memory subscription index (segment tree) rebuilt on startup from ArangoDB
- `DeleteSubscription` sets status to `cancelled`; delivery loop skips cancelled subscriptions

See: [mvp-details/subscribe.md](mvp-details/subscribe.md)

---

## PUBSUB-004 — Event Replay / Query API

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-004: Implement `QueryEvents(ctx, QueryEventsRequest) ([]Event, error)` — ArangoDB query with pattern matching, time range, pagination | 📋 Not Started | PUBSUB-002 |

**Scope**:
- `QueryEventsRequest` fields: `Pattern string`, `AgencyID string`, `From time.Time`, `To time.Time`, `Limit int`, `Cursor string`
- AQL query against `PubSubEvent` collection filtered by agencyID and time range
- Post-filter by topic pattern (segment-tree matching) if pattern contains wildcards
- Return results ordered by `PublishedAt` ascending; include next-page cursor

See: [mvp-details/query.md](mvp-details/query.md)

---

## PUBSUB-005 — Delivery Loop & ACK

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-005: Background delivery loop — push pending `PubSubDelivery` records to subscriber endpoints; exponential backoff retry; `Ack` implementation | 📋 Not Started | PUBSUB-003 |

**Scope**:
- Delivery loop runs as a goroutine in the service process
- Polls `PubSubDelivery` where `status = pending` and `last_attempt_at < now - retry_interval`
- Pushes event to subscriber's `DeliveryURL` via gRPC call
- On success: updates delivery status to `delivered`
- On `Ack` call: updates status to `acked`; stops retrying
- On repeated failure: exponential backoff (1 s → 2 s → 4 s … capped at 5 min); after max attempts, status → `failed`

See: [mvp-details/delivery.md](mvp-details/delivery.md)

---

## PUBSUB-006 — gRPC Proto & Server

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-006a: Define `proto/codevaldpubsub/v1/service.proto` — `PubSubService` with Publish, Subscribe, ListSubscriptions, GetSubscription, DeleteSubscription, Ack, QueryEvents RPCs | 📋 Not Started | PUBSUB-001 |
| PUBSUB-006b: Generate Go gRPC code via `buf generate`; implement gRPC handlers in `internal/server/server.go`; proto ↔ domain mappers in `internal/server/mappers.go` | 📋 Not Started | PUBSUB-006a |

See: [mvp-details/grpc.md](mvp-details/grpc.md)

---

## PUBSUB-007 — CodeValdCross Integration

| Task | Status | Depends On |
|------|--------|------------|
| PUBSUB-007: Implement `internal/registrar/registrar.go` — register 7 HTTP routes with CodeValdCross via 20-second heartbeat; wire `cmd/server/main.go` with ArangoDB, PubSubManager, delivery loop, and gRPC server | 📋 Not Started | PUBSUB-006b |

**Scope**: Mirrors the integration pattern from CodeValdGit's `internal/registrar`. Routes to register:

| HTTP | gRPC | Description |
|---|---|---|
| `POST /{agencyID}/events` | `Publish` | Publish an event |
| `POST /{agencyID}/subscriptions` | `Subscribe` | Create a subscription |
| `GET /{agencyID}/subscriptions` | `ListSubscriptions` | List subscriptions |
| `GET /{agencyID}/subscriptions/{id}` | `GetSubscription` | Get one subscription |
| `DELETE /{agencyID}/subscriptions/{id}` | `DeleteSubscription` | Cancel a subscription |
| `POST /{agencyID}/subscriptions/{id}/ack` | `Ack` | Acknowledge event delivery |
| `GET /{agencyID}/events` | `QueryEvents` | Replay historical events |

See: [mvp-details/integration.md](mvp-details/integration.md)
