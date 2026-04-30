# CodeValdPubSub — Architecture

## 1. Core Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Storage | ArangoDB via `entitygraph.DataManager` | Events are first-class entities in the platform graph; queryable, durable, consistent with platform storage model |
| Transport | gRPC + CodeValdCross HTTP proxy | Same integration pattern as CodeValdWork, CodeValdGit, CodeValdAgency |
| Topic format | `<service>.<agencyID>.<entity-segments…>.<action>` | Hierarchical; agencyID in position 2 enables cross-service agency scoping (`*.<agencyID>.#`) |
| Subscription matching | Segment wildcard `*` + multi-segment suffix `#` | Flexible routing without full regex overhead; compiled to segment-tree index at startup |
| Delivery model | Fan-out to all matching subscribers | Publisher does not enumerate consumers; each subscriber is independent |
| Delivery guarantee | At-least-once | Durable record + retry; subscribers must be idempotent |
| Intra-topic ordering | Preserved per subscriber | Events within a topic are appended in publish order; cross-topic ordering is not guaranteed |
| Idempotent publish | 24-hour `IdempotencyKey` window | Prevents duplicate events on publisher retry; key is (agencyID, topic, idempotencyKey) |
| Service scoping | `PubSubManager` scoped to one agencyID at construction | Matches the pattern established by `GitManager` and `WorkManager` |
| Service registration | Heartbeat registrar to Cross every 20 s | Consistent with all other platform services |

---

## 2. PubSubManager Interface

```go
// PubSubManager is the primary interface for the CodeValdPubSub library.
// Each instance is scoped to one agency at construction time.
type PubSubManager interface {
    // Publish records an event durably and routes it to all matching subscribers.
    // The event's topic must conform to the hierarchical format:
    //   <service>.<agencyID>.<entity-segments…>.<action>
    // Returns the recorded Event including its assigned ID and Timestamp.
    Publish(ctx context.Context, req PublishRequest) (Event, error)

    // Subscribe registers a topic pattern. The subscriber will receive all
    // future events whose topic matches the pattern via the delivery loop.
    // Wildcard: * matches one segment; # matches remaining suffix.
    Subscribe(ctx context.Context, req SubscribeRequest) (Subscription, error)

    // ListSubscriptions returns all active subscriptions for the agency.
    ListSubscriptions(ctx context.Context) ([]Subscription, error)

    // GetSubscription returns a single subscription by ID.
    GetSubscription(ctx context.Context, subscriptionID string) (Subscription, error)

    // DeleteSubscription cancels a subscription. Undelivered events for this
    // subscription are discarded.
    DeleteSubscription(ctx context.Context, subscriptionID string) error

    // Ack acknowledges delivery of an event to a subscription. PubSub stops
    // retrying this (subscriptionID, eventID) pair.
    Ack(ctx context.Context, subscriptionID, eventID string) error

    // QueryEvents returns historical events matching the given filter, ordered
    // by timestamp ascending (oldest first).
    QueryEvents(ctx context.Context, req QueryEventsRequest) ([]Event, error)
}

func NewPubSubManager(dm DataManager, sm SchemaManager, agencyID string) PubSubManager
```

---

## 3. Topic Format

### Specification

```
<service>.<agencyID>.<entity-segment-1>…<entity-segment-N>.<action>
```

- **service** — the originating service name (`work`, `git`, `agency`, `comm`, `dt`)
- **agencyID** — the owning agency; always position 2
- **entity-segments** — zero or more resource identifiers; use human-readable names where stable (project name, task name), entity IDs otherwise (repo ID, branch ID)
- **action** — the terminal segment describing the occurrence; past-tense or gerund verb

### Constraints

- Minimum 3 segments (`<service>.<agencyID>.<action>`)
- Maximum 10 segments
- Maximum 512 characters
- Segments may contain: `[a-zA-Z0-9_-]` and dots only within the action for compound actions (`conflict.detected`, `import.failed`)

### Examples

```
work.agency-abc.project-x.task-001.createbranch
work.agency-abc.project-x.task-001.completed
git.agency-abc.repo-001.branch-042.merged
git.agency-abc.repo-001.branch-042.conflict.detected
agency.agency-abc.created
agency.agency-abc.published
```

---

## 4. Subscription Pattern Matching

Patterns are compiled into a segment-tree index at subscription creation time.

| Wildcard | Semantics |
|---|---|
| `*` | Matches exactly one segment, any value |
| `#` | Matches any remaining segments; must be the final pattern token |

Pattern matching is deterministic: a topic either matches or does not. If a topic matches multiple subscriptions held by the same subscriber, the subscriber receives one delivery per matching subscription.

### Examples

| Pattern | Matches | Does not match |
|---|---|---|
| `work.agency-abc.#` | `work.agency-abc.project-x.task-001.created` | `git.agency-abc.repo-001.created` |
| `work.*.*.*.createbranch` | `work.agency-xyz.project-a.task-1.createbranch` | `work.agency-xyz.project-a.task-1.task-2.createbranch` |
| `git.*.*.*.merged` | `git.agency-abc.repo-001.branch-042.merged` | `git.agency-abc.repo-001.merged` |
| `#` | Any topic | — |

---

## 5. Storage Schema

Events and subscriptions are stored as entitygraph entities in ArangoDB. The schema is registered via `DefaultPubSubSchema()`.

### Entity Types

| Type | Collection | Description |
|---|---|---|
| `PubSubEvent` | `pubsub_events` | One record per published event; immutable after write |
| `PubSubSubscription` | `pubsub_subscriptions` | One record per registered subscription; mutable (can be cancelled) |
| `PubSubDelivery` | `pubsub_deliveries` | One record per (subscription, event) delivery attempt; updated on ACK |

### PubSubEvent fields

| Field | Type | Description |
|---|---|---|
| `ID` | string | UUID v4; globally unique |
| `AgencyID` | string | Owning agency |
| `Topic` | string | Full hierarchical topic |
| `Service` | string | Publishing service (first topic segment) |
| `Payload` | any | Service-specific payload struct |
| `IdempotencyKey` | string | Publisher-supplied dedup key; (AgencyID, Topic, IdempotencyKey) unique within 24 h |
| `PublishedAt` | time.Time | UTC timestamp; set by PubSub on write |

### PubSubSubscription fields

| Field | Type | Description |
|---|---|---|
| `ID` | string | UUID v4 |
| `AgencyID` | string | Owning agency |
| `Pattern` | string | Topic pattern (may contain `*` and `#`) |
| `DeliveryURL` | string | gRPC endpoint where events are pushed (push model) |
| `Status` | string | `active` or `cancelled` |
| `CreatedAt` | time.Time | — |

### PubSubDelivery fields

| Field | Type | Description |
|---|---|---|
| `ID` | string | UUID v4 |
| `SubscriptionID` | string | Foreign key to `PubSubSubscription` |
| `EventID` | string | Foreign key to `PubSubEvent` |
| `Status` | string | `pending`, `delivered`, `acked`, `failed` |
| `AttemptCount` | int | Number of delivery attempts |
| `LastAttemptAt` | time.Time | UTC; used by retry loop |
| `AckedAt` | *time.Time | Set when subscriber calls `Ack` |

---

## 6. Delivery Flow

```
Publisher → Publish(req)
              │
              ▼
         Write PubSubEvent (ArangoDB)
              │
              ▼
         Match topic against subscription index
              │
              ├──► Subscription A → Write PubSubDelivery (pending) → push to subscriber
              ├──► Subscription B → Write PubSubDelivery (pending) → push to subscriber
              └──► Subscription C → Write PubSubDelivery (pending) → push to subscriber
                                                 │
                                          Subscriber calls Ack
                                                 │
                                          Update PubSubDelivery → acked
```

Deliveries that are not acknowledged within the retry timeout are re-attempted by a background retry loop. The retry loop uses exponential backoff capped at a configurable maximum interval.

---

## 7. CodeValdCross Integration

PubSub registers its HTTP routes with CodeValdCross via the standard heartbeat registrar (every 20 seconds). All routes are documented in the registrar; Cross requires no recompile when routes are added.

### gRPC Service Endpoints

| HTTP (Cross proxy) | gRPC | Description |
|---|---|---|
| `POST /{agencyID}/events` | `Publish` | Publish an event |
| `POST /{agencyID}/subscriptions` | `Subscribe` | Create a subscription |
| `GET /{agencyID}/subscriptions` | `ListSubscriptions` | List subscriptions |
| `GET /{agencyID}/subscriptions/{id}` | `GetSubscription` | Get one subscription |
| `DELETE /{agencyID}/subscriptions/{id}` | `DeleteSubscription` | Cancel a subscription |
| `POST /{agencyID}/subscriptions/{id}/ack` | `Ack` | Acknowledge event delivery |
| `GET /{agencyID}/events` | `QueryEvents` | Replay historical events |
