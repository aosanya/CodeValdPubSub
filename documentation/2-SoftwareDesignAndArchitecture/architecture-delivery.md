# CodeValdPubSub — Delivery Guarantees

## 1. Delivery Model

PubSub uses a **push delivery** model: when an event is published, PubSub proactively pushes it to each matching subscriber's `DeliveryURL` gRPC endpoint. Subscribers do not poll.

---

## 2. At-Least-Once Guarantee

PubSub guarantees **at-least-once** delivery for every event to every matching subscriber. It does not guarantee exactly-once delivery. Subscribers must be idempotent and deduplicate by `Event.ID`.

The guarantee holds as long as:
1. The subscriber's `DeliveryURL` endpoint is reachable within the max retry window
2. The subscriber eventually calls `Ack` (or the delivery succeeds without error) before max attempts are exhausted

If a subscriber's endpoint is unreachable for longer than the max retry window (configurable; default: 24 hours), the delivery record transitions to `failed` and no further retries occur. The event remains recorded in `pubsub_events` and can be replayed via `QueryEvents`.

---

## 3. Retry Schedule

| Attempt | Wait before retry |
|---------|------------------|
| 1st retry | 1 s |
| 2nd retry | 2 s |
| 3rd retry | 4 s |
| 4th retry | 8 s |
| 5th retry | 16 s |
| … | … |
| Cap | 300 s (5 min) |

The delivery loop polls `pubsub_deliveries` for eligible records every 10 seconds.

---

## 4. Acknowledgement

A subscriber acknowledges receipt by calling:

```
POST /{agencyID}/subscriptions/{subscriptionID}/ack
Body: { "event_id": "<uuid>" }
```

On ACK:
- `PubSubDelivery.status` → `acked`
- `PubSubDelivery.acked_at` → current UTC timestamp
- No further retries for this (subscriptionID, eventID) pair

Implicit acknowledgement: if the delivery push call returns a success gRPC status code, PubSub transitions the delivery to `delivered` (not `acked`). The `delivered` state still retries until explicit `Ack` is received. This distinction allows subscribers that process events asynchronously to defer ACK until processing is complete.

---

## 5. Ordering

Events within a single topic are delivered to each subscriber in the order they were published (ascending `PublishedAt`). This ordering is maintained per-subscription: two subscriptions with overlapping patterns may receive events in different interleaved orders relative to each other, but each subscription sees its own topic's events in order.

Cross-topic ordering is not guaranteed.

---

## 6. Deduplication

Each `PubSubEvent` has a globally unique `ID` (UUID v4) assigned by PubSub at record time. Subscribers must use this ID for deduplication — storing a set of processed IDs or using an idempotency key derived from the event ID.

Publishers may supply an `IdempotencyKey` to prevent duplicate event recording on their own retries. The PubSub service deduplicates at the record level: if `(agencyID, topic, idempotencyKey)` already exists within 24 hours, the existing event is returned and no new deliveries are created.

---

## 7. Dead Letter Handling

When a delivery reaches `failed` status (max attempts exhausted), the event is not re-delivered automatically. Options for recovery:
- Use `QueryEvents` to replay the event manually
- Re-publish the event from the original publisher (if idempotent)
- Operator inspection of `pubsub_deliveries` where `status = "failed"`

A dead-letter subscription (a subscription that receives `failed` delivery notifications) is planned but not in scope for MVP.
