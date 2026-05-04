# CodeValdPubSub — Event Delivery & Acknowledgement

## Overview

When an event is published, PubSub creates a per-subscription `Delivery` record
and exposes `Ack` + `GetSubscribersForTopic` RPCs. Cross drives the actual push
to consumer services and calls `Ack` on success. PubSub's role is durable
storage and delivery state — it never dials consumer services.

---

## 1. Confirmed Design Decisions

| Decision | Choice |
|---|---|
| Who pushes events to consumers | CodeValdCross — not PubSub |
| Who calls `Ack` | CodeValdCross, immediately after a successful `NotifyEvent` response |
| On `NotifyEvent` failure | Cross does nothing — delivery stays `pending`; retry deferred |
| Subscribe registration | Cross subscribes on behalf of services; called on every heartbeat; idempotent on `(subscriber_service, topic_pattern)` |
| Consumer never dials PubSub | All interaction goes via Cross |

---

## 2. Schema Addition — `Delivery` Entity

Add a fourth entity type to `schema.go` alongside `Topic`, `Event`, `Subscription`.

```go
{
    Name:              "Delivery",
    DisplayName:       "Delivery",
    PathSegment:       "deliveries",
    EntityIDParam:     "deliveryId",
    StorageCollection: "pubsub_deliveries",
    Properties: []types.PropertyDefinition{
        {Name: "subscription_id",   Type: types.PropertyTypeString, Required: true},
        {Name: "event_id",          Type: types.PropertyTypeString, Required: true},
        // status: "pending" | "delivered" | "acked" | "failed"
        {Name: "status",            Type: types.PropertyTypeString, Required: true},
        {Name: "attempt_count",     Type: types.PropertyTypeNumber},
        {Name: "last_attempted_at", Type: types.PropertyTypeString},
        {Name: "acked_at",          Type: types.PropertyTypeString},
        {Name: "created_at",        Type: types.PropertyTypeString},
        {Name: "updated_at",        Type: types.PropertyTypeString},
    },
    Relationships: []types.RelationshipDefinition{
        {Name: "delivery_for_event",        ToType: "Event",        ToMany: false, Inverse: "has_delivery"},
        {Name: "delivery_for_subscription", ToType: "Subscription", ToMany: false, Inverse: "has_delivery"},
    },
},
```

Add inverse `has_delivery` relationships on both `Event` and `Subscription`.

### Updated graph topology

```
Event        ──for_topic──────────► Topic
Event        ──has_delivery───────► Delivery
Subscription ──subscribes_to──────► Topic
Subscription ──has_delivery───────► Delivery
```

---

## 3. Domain Model Addition (`models.go`)

```go
type Delivery struct {
    ID              string
    SubscriptionID  string
    EventID         string
    Status          string // "pending" | "delivered" | "acked" | "failed"
    AttemptCount    int
    LastAttemptedAt string // RFC3339; empty until first attempt
    AckedAt         string // RFC3339; empty until Ack is called
    CreatedAt       string
    UpdatedAt       string
}

type AckRequest struct {
    SubscriptionID string
    EventID        string
}
```

---

## 4. Manager Interface Extension (`pubsub.go`)

```go
// Ack records that Cross has confirmed delivery for (subscriptionID, eventID).
// Sets Delivery.Status → "acked", Delivery.AckedAt → now.
// Idempotent: calling Ack on an already-acked delivery is a no-op.
Ack(ctx context.Context, agencyID string, req AckRequest) error

// GetSubscribersForTopic returns all active Subscriptions whose TopicPattern
// matches the given topic string. Called by Cross after every Publish to
// determine which consumer services to notify.
GetSubscribersForTopic(ctx context.Context, agencyID, topic string) ([]Subscription, error)

// RecordDelivery creates a Delivery in "pending" state for (subscriptionID, eventID).
// Called internally by RecordEvent after finding matching subscriptions.
RecordDelivery(ctx context.Context, agencyID, subscriptionID, eventID string) (Delivery, error)

// MarkDelivered transitions a Delivery from "pending" to "delivered".
// Called by Cross after a successful NotifyEvent push.
MarkDelivered(ctx context.Context, agencyID, deliveryID string) error
```

---

## 5. Idempotent Subscribe

`Subscribe` must be idempotent on `(subscriber_service, topic_pattern)`:

- If a `Subscription` with the same `subscriber_service` + `topic_pattern` already
  exists for the agency, return the existing record instead of creating a duplicate
- Cross calls `Subscribe` on every heartbeat — this is by design, not a bug
- A `UniqueKey: []string{"subscriber_service", "topic_pattern"}` constraint in the
  schema enforces this at the storage layer

---

## 6. Proto / gRPC Changes (`service.proto`)

### New message types

```protobuf
message Delivery {
  string                    id              = 1;
  string                    subscription_id = 2;
  string                    event_id        = 3;
  string                    status          = 4;
  int32                     attempt_count   = 5;
  google.protobuf.Timestamp last_attempted_at = 6;
  google.protobuf.Timestamp acked_at        = 7;
  google.protobuf.Timestamp created_at      = 8;
}

message AckRequest {
  string agency_id       = 1;
  string subscription_id = 2;
  string event_id        = 3;
}

message AckResponse {}

message GetSubscribersForTopicRequest {
  string agency_id = 1;
  string topic     = 2;
}

message GetSubscribersForTopicResponse {
  repeated Subscription subscriptions = 1;
}
```

### New RPCs

```protobuf
rpc Ack(AckRequest) returns (AckResponse);
rpc GetSubscribersForTopic(GetSubscribersForTopicRequest)
    returns (GetSubscribersForTopicResponse);
```

---

## 7. RecordEvent → Delivery Creation

After writing the `Event` entity, `RecordEvent` creates a `Delivery("pending")`
for each matching subscription:

```
RecordEvent(ctx, agencyID, req)
  1. Write Event entity → eventID
  2. GetSubscribersForTopic(agencyID, req.Topic) → []Subscription
  3. for each sub:
       RecordDelivery(agencyID, sub.ID, eventID) → status: "pending"
  4. Return Event
```

Cross reads these records via `GetSubscribersForTopic`, pushes to consumers,
then calls `Ack` (or does nothing on failure — delivery stays `pending`).

---

## 8. Ack Flow

```
Cross → Ack(agencyID, subscriptionID, eventID)
  1. Find Delivery WHERE (subscription_id, event_id)
  2. If status == "acked" → return OK (idempotent)
  3. Update: status → "acked", acked_at → now
```

Error cases: `ErrDeliveryNotFound`, `ErrSubscriptionNotFound`.

---

## 9. Error Types

Add to `errors.go`:

```go
var ErrDeliveryNotFound = errors.New("delivery not found")
```

---

## 10. Storage Notes

- `(subscription_id, event_id)` compound unique index on `pubsub_deliveries`
- `Ack` wrapped in an ArangoDB transaction to prevent lost-update on concurrent calls
- `GetSubscribersForTopic` queries `pubsub_subscriptions` WHERE `status == "active"`
  and `topic_pattern == topic` (exact match for MVP; wildcard in follow-on)
- `Subscribe` uses `UniqueKey: ["subscriber_service", "topic_pattern"]` for idempotency

---

## 11. Definition of Done

- [ ] `Delivery` entity added to `schema.go` with `UniqueKey` on `(subscription_id, event_id)`
- [ ] `Subscribe` updated — idempotent on `(subscriber_service, topic_pattern)`; `UniqueKey` added to schema
- [ ] `Delivery`, `AckRequest` types added to `models.go`
- [ ] `Ack`, `GetSubscribersForTopic`, `RecordDelivery`, `MarkDelivered` added to Manager + implemented
- [ ] `RecordEvent` creates `Delivery("pending")` records for each matching subscription
- [ ] Proto updated; `buf generate` run; `internal/server/server.go` wires new RPCs
- [ ] `ErrDeliveryNotFound` added and mapped to gRPC `NotFound`
- [ ] Unit tests: `Ack` (success, idempotent, not found), `GetSubscribersForTopic` (match, no match), idempotent `Subscribe`
