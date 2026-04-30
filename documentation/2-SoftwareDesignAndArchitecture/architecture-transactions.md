# CodeValdPubSub — Atomicity and Idempotency

## Publish Atomicity

`Publish` must write the `PubSubEvent` record to ArangoDB **before** creating any `PubSubDelivery` records or pushing to subscribers. If the event write fails, the call returns an error and no side effects occur.

If the event write succeeds but delivery record creation partially fails (network partition mid-fan-out), the delivery loop will not pick up deliveries that were never created. This is a known gap in v1: delivery records that fail to write are not retried. In practice, ArangoDB writes within the same service process are highly reliable; this gap is accepted for MVP.

## Idempotent Publish

Publishers may supply an `IdempotencyKey` (string, max 128 chars) in `PublishRequest`. PubSub checks for an existing `PubSubEvent` where `(agency_id, topic, idempotency_key)` matches and `published_at > now - 24h` before inserting. If found, the existing event is returned and no new deliveries are created.

The idempotency window is 24 hours. After 24 hours, the same key can be used again for a new event.

Publishers that do not supply an idempotency key get no deduplication — duplicate publishes create duplicate events and deliveries. Publishers should always supply an idempotency key when their publish call may be retried (network errors, process restarts).

## ACK Idempotency

`Ack(subscriptionID, eventID)` is idempotent. Calling it multiple times for the same pair is a no-op after the first successful ACK.
