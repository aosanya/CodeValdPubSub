# CodeValdPubSub — Requirements

## 1. Purpose

CodeValdPubSub is a **Go gRPC microservice** that provides durable pub/sub event recording and routing for the CodeVald platform.

Platform services (CodeValdWork, CodeValdGit, CodeValdAgency, etc.) publish lifecycle events to CodeValdPubSub. PubSub records every event durably in ArangoDB and routes it to registered subscribers. Subscribers may also replay historical events by topic pattern and time range.

---

## 2. Scope

### In Scope
- Durable recording of all published events in ArangoDB
- Hierarchical topic routing with wildcard pattern subscription
- Fan-out delivery to multiple independent subscribers per topic pattern
- At-least-once delivery with subscriber acknowledgement and retry
- Historical event replay by topic pattern, agencyID, and time range
- gRPC service exposed via CodeValdCross HTTP proxy

### Out of Scope
- Exactly-once delivery (subscribers must be idempotent)
- Cross-topic ordering guarantees
- Payload transformation or schema validation
- Authorization enforcement (deferred until CodeValdOrg lands)
- Arbitrary binary payloads (events carry structured CodeVald entity data)

---

## 3. Functional Requirements

### FR-001: Durable Event Recording
- Every call to `Publish` must write the event to ArangoDB **before** routing it to any subscriber
- Publish must be atomic with respect to the event record: if the write fails, no routing occurs
- Events are never silently dropped — failures are returned to the publisher

### FR-002: Hierarchical Topic Format
- Every event topic must conform to the format: `<service>.<agencyID>.<entity-segment-1>…<entity-segment-N>.<action>`
- The service segment is the originating service name (`work`, `git`, `agency`, `comm`, `dt`)
- The agencyID segment is always the second segment
- Entity segments are zero or more resource identifiers (project name, task name, repo ID, branch ID, etc.) between the agencyID and the action
- The action is the final segment — a past-tense or gerund verb naming the occurrence (`created`, `completed`, `merged`, `createbranch`, `conflict.detected`)
- Topics may not exceed 10 segments or 512 characters

### FR-003: Pattern Subscriptions
- Subscribers register a **topic pattern** at subscription creation time
- Pattern syntax:
  - `.` separates segments
  - `*` matches exactly one segment (any value)
  - `#` matches any remaining suffix (must be the last pattern segment)
- Subscriptions persist across service restarts
- A subscriber may hold multiple subscriptions; each is matched independently

### FR-004: Fan-Out Delivery
- When an event is published, PubSub must route a copy to **every** subscriber whose pattern matches the topic
- Delivery to one subscriber must not block or fail delivery to others
- Delivery is best-effort concurrent; ordering within a topic is preserved per-subscriber

### FR-005: At-Least-Once Delivery
- PubSub must retry delivery to a subscriber that has not acknowledged an event within a configurable timeout
- Each event is assigned a globally unique ID (UUID v4); subscribers use this ID for deduplication
- A subscriber may acknowledge an event explicitly (via `Ack`) or implicitly by returning success from its handler gRPC method

### FR-006: Event Replay
- The service must expose a `QueryEvents` API that returns historical events matching:
  - A topic pattern (same wildcard syntax as subscriptions)
  - An optional agencyID filter
  - An optional time range (`from`, `to`)
  - A limit and pagination cursor
- Results are returned ordered by event timestamp, oldest first

### FR-007: CodeValdCross Integration
- PubSub must register its HTTP routes and gRPC method bindings with CodeValdCross via the standard heartbeat registrar (every 20 seconds)
- No Cross recompile is required when routes are added

### FR-008: Agency Isolation
- All operations (publish, subscribe, query) are scoped to a single agencyID — the owning agency of the caller's context
- A subscriber registered to `work.agency-abc.#` receives only events whose agencyID segment is `agency-abc`
- Cross-agency subscriptions are not permitted in v1

---

## 4. Non-Functional Requirements

### NFR-001: Embeddable Go Library
- The core PubSub logic must be importable as a standard Go module
- The gRPC server is a thin shell over the library; the library itself has no daemon dependency
- Storage backend is injected via `entitygraph.DataManager`

### NFR-002: Concurrent Safety
- `PubSubManager` implementations must be safe for concurrent use without external locking
- Event fan-out must not serialize subscriber delivery

### NFR-003: Idempotent Publish
- Publishing the same event twice (same `IdempotencyKey`) must record one event and return the same result both times
- The idempotency window is at least 24 hours

---

## 5. Open Questions

| # | Question | Impact |
|---|---|---|
| OQ-001 | How does PubSub deliver events to subscribers — push (gRPC server-stream) or pull (subscriber polls)? | Architecture of delivery loop |
| OQ-002 | What is the configurable retry timeout for unacknowledged events? | SLA for at-least-once |
| OQ-003 | What is the default event retention period? | Storage sizing |
| OQ-004 | Should cross-agency subscriptions be permitted for operator-level callers? | Permission model |
