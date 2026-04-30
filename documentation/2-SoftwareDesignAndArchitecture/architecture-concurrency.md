# CodeValdPubSub — Concurrency Model

## 1. Concurrent Fan-Out

When `Publish` is called, PubSub fans out delivery to all matching subscriptions. Deliveries to different subscribers are dispatched concurrently — delivery to subscriber A does not block delivery to subscriber B.

Each `PubSubDelivery` record is written to ArangoDB before the push attempt. This ensures that even if the push goroutine is interrupted mid-flight, the delivery loop will retry from the durable record.

---

## 2. Subscription Index Concurrency

The in-memory subscription index (`router.Index`) is read by concurrent `Publish` calls and written by `Subscribe` and `DeleteSubscription`. The index is protected by a `sync.RWMutex`:

- `Match(topic)` → `RLock` (concurrent reads allowed)
- `Add(pattern, id)` → `Lock` (exclusive write)
- `Remove(id)` → `Lock` (exclusive write)

Write operations on the index are fast (segment-tree insert/delete) and do not block reads for more than microseconds.

---

## 3. Delivery Loop Isolation

The delivery loop runs as a single background goroutine. It does not share mutable state with the gRPC handler goroutines beyond the ArangoDB collections (which provide their own transaction isolation). The loop reads from `pubsub_deliveries` using an AQL query with `FOR … FILTER … LIMIT` to bound each poll batch.

The loop does not hold locks on the subscription index. If a subscription is cancelled between when the loop reads a delivery record and when it attempts the push, the push is attempted (and may fail); the cancellation is detected on the next retry when the subscription status is checked.

---

## 4. PubSubManager Safety

`PubSubManager` implementations must be safe for concurrent use. The concrete implementation holds:
- An `entitygraph.DataManager` (concurrency-safe by contract)
- A `router.Index` (protected by `sync.RWMutex`)
- No other shared mutable state

All methods acquire only the locks they need for the minimum required duration. No method holds both the subscription index lock and an ArangoDB write simultaneously.
