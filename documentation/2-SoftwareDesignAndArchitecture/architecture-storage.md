# CodeValdPubSub — ArangoDB Storage Design

## 1. Storage Approach

Events, subscriptions, and delivery records are stored as entitygraph entities in ArangoDB via `entitygraph.DataManager`. This is consistent with how CodeValdWork, CodeValdGit, and CodeValdAgency store their domain objects — no separate storage layer is introduced.

The schema is registered at startup via `DefaultPubSubSchema()` and seeded with `entitygraph.SeedSchema`.

---

## 2. Collections

| Collection | Entity Type | Mutable? |
|---|---|---|
| `pubsub_events` | `PubSubEvent` | No — events are immutable after write |
| `pubsub_subscriptions` | `PubSubSubscription` | Yes — status can be updated to `cancelled` |
| `pubsub_deliveries` | `PubSubDelivery` | Yes — status and attempt count updated by delivery loop |

All collections are named per the `TypeDefinition.StorageCollection` field in `DefaultPubSubSchema()`.

---

## 3. PubSubEvent Schema

```
PubSubEvent
├── id                string   UUID v4; globally unique
├── agency_id         string   Owning agency
├── topic             string   Full hierarchical topic
├── service           string   First topic segment (publisher service name)
├── payload           object   Service-specific payload (stored as JSON)
├── idempotency_key   string   Publisher-supplied dedup key (may be empty)
├── published_at      string   RFC 3339 UTC timestamp; set by PubSub on write
└── schema_version    string   "pubsub-schema-v1"
```

**Unique constraint**: `(agency_id, topic, idempotency_key)` where `idempotency_key != ""` — enforced at write time; the writer checks for an existing record within 24 hours before inserting.

Events are never deleted. Retention policy (if any) is handled by an operator-run archival job, not by the service itself.

---

## 4. PubSubSubscription Schema

```
PubSubSubscription
├── id              string   UUID v4
├── agency_id       string   Owning agency
├── pattern         string   Topic pattern (may contain * and #)
├── delivery_url    string   gRPC endpoint for push delivery
├── status          string   "active" | "cancelled"
├── created_at      string   RFC 3339 UTC
└── cancelled_at    string   RFC 3339 UTC; set when DeleteSubscription is called
```

On service startup, all `active` subscriptions are loaded into the in-memory subscription index. The index is rebuilt on each startup — no incremental sync.

---

## 5. PubSubDelivery Schema

```
PubSubDelivery
├── id                 string    UUID v4
├── subscription_id    string    Foreign key to PubSubSubscription.id
├── event_id           string    Foreign key to PubSubEvent.id
├── agency_id          string    Owning agency (denormalized for query efficiency)
├── status             string    "pending" | "delivered" | "acked" | "failed"
├── attempt_count      int       Number of delivery attempts
├── last_attempt_at    string    RFC 3339 UTC; null if never attempted
├── acked_at           string    RFC 3339 UTC; null until ACK received
└── error_message      string    Last error from delivery attempt; empty on success
```

The delivery loop queries `pubsub_deliveries` where `status = "pending"` and `last_attempt_at < (now - retry_interval)`. Retry intervals use exponential backoff: 1 s, 2 s, 4 s, 8 s … capped at 300 s.

---

## 6. In-Memory Subscription Index

The subscription index is a **segment-tree** built from all active `PubSubSubscription.Pattern` values. It is rebuilt at service startup and updated incrementally as subscriptions are created or cancelled.

```go
// router.Index is the in-memory pattern matcher.
type Index interface {
    // Add registers a pattern → subscriptionID mapping.
    Add(pattern, subscriptionID string) error
    // Remove deregisters a subscriptionID.
    Remove(subscriptionID string)
    // Match returns all subscriptionIDs whose pattern matches topic.
    Match(topic string) []string
}
```

Matching is O(S × D) where S is the number of unique segments in the topic and D is the max depth of the pattern tree. For typical CodeVald topics (3–6 segments), this is effectively constant time.
