# 2 — Software Design & Architecture

## Overview

This section captures the **how** — design decisions, data models, component architecture, and technical constraints for CodeValdPubSub.

---

## Index

| Document | Description |
|---|---|
| [architecture.md](architecture.md) | Core design decisions, topic naming, routing engine, storage schema, API interface, CodeValdCross integration |
| [architecture-storage.md](architecture-storage.md) | ArangoDB entitygraph schema — `PubSubEvent`, `PubSubSubscription`, `PubSubDelivery` collections and fields |
| [architecture-delivery.md](architecture-delivery.md) | Delivery guarantees — at-least-once, push model, retry schedule, ACK, deduplication, dead-letter handling |
| [architecture-concurrency.md](architecture-concurrency.md) | Concurrency model — concurrent fan-out, subscription index locking, delivery loop isolation |
| [architecture-transactions.md](architecture-transactions.md) | Atomicity rules — publish atomicity, idempotent publish, ACK idempotency |

---

## Key Design Decisions at a Glance

| Decision | Choice | Rationale |
|---|---|---|
| Storage | ArangoDB via `entitygraph.DataManager` | Events are entities; queryable, durable, consistent with platform storage model |
| Transport | gRPC + CodeValdCross HTTP proxy | Same integration pattern as CodeValdWork, CodeValdGit, CodeValdAgency |
| Topic format | `<service>.<agencyID>.<entity-segments…>.<action>` | AgencyID in position 2 enables cross-service agency scoping (`*.<agencyID>.#`) |
| Subscription matching | Segment wildcard `*`, multi-segment suffix `#` | Compiled segment-tree index; O(S × D) match per topic |
| Delivery model | Push fan-out to all matching subscribers | Publisher does not enumerate consumers |
| Delivery guarantee | At-least-once | Durable record + exponential backoff retry; subscribers must be idempotent |
| Intra-topic ordering | Preserved per subscriber | Events within a topic delivered in publish order |
| Idempotent publish | 24-hour `IdempotencyKey` window | Deduplication key: (agencyID, topic, idempotencyKey) |
| Service scoping | `PubSubManager` scoped to one agencyID at construction | Matches CodeValdGit `GitManager` and CodeValdWork `WorkManager` patterns |
| Service registration | Heartbeat registrar to Cross every 20 s | Consistent with all other platform services |

---

## Component Architecture

```
github.com/aosanya/CodeValdPubSub    ← root package (library entry point)
├── pubsub.go                         # PubSubManager interface + NewPubSubManager
├── models.go                         # Event, Subscription, DeliveryRecord value types
├── types.go                          # PublishRequest, SubscribeRequest, QueryEventsRequest
├── schema.go                         # DefaultPubSubSchema() — entity types for entitygraph
├── errors.go                         # ErrTopicInvalid, ErrSubscriptionNotFound, ErrDuplicateEvent
└── internal/
    ├── server/                       # Inbound gRPC server
    │   ├── server.go                 # PubSubService gRPC handlers
    │   └── mappers.go                # Proto ↔ domain model conversion
    ├── router/                       # Topic pattern matching and fan-out
    │   └── router.go                 # Segment-tree pattern index + Match()
    └── registrar/                    # CodeValdCross heartbeat
        └── registrar.go
```

---

## gRPC Service Endpoints

| HTTP (Cross proxy) | gRPC | Description |
|---|---|---|
| `POST /{agencyID}/events` | `Publish` | Publish an event |
| `POST /{agencyID}/subscriptions` | `Subscribe` | Create a subscription |
| `GET /{agencyID}/subscriptions` | `ListSubscriptions` | List subscriptions |
| `GET /{agencyID}/subscriptions/{id}` | `GetSubscription` | Get one subscription |
| `DELETE /{agencyID}/subscriptions/{id}` | `DeleteSubscription` | Cancel a subscription |
| `POST /{agencyID}/subscriptions/{id}/ack` | `Ack` | Acknowledge event delivery |
| `GET /{agencyID}/events` | `QueryEvents` | Replay historical events |
