# High-Level Features

## Feature Overview

CodeValdPubSub provides the following top-level capabilities to the CodeVald platform:

---

### 1. Durable Event Recording

Every event published to CodeValdPubSub is written to ArangoDB before any routing occurs. Events survive service restarts. No event is ever in-memory-only.

---

### 2. Hierarchical Topic Routing

Topics follow a dot-separated hierarchy that embeds the agency and entity context:

```
<service>.<agencyID>.<entity-segment-1>.<entity-segment-2>….<action>
```

Examples:
```
work.agency-abc.project-x.task-001.createbranch
work.agency-abc.project-x.task-001.completed
git.agency-abc.repo-001.branch-042.merged
git.agency-abc.repo-001.branch-042.conflict.detected
agency.agency-abc.created
agency.agency-abc.published
```

The agencyID is always the second segment. This makes agency-scoped subscriptions a first-class pattern.

---

### 3. Pattern Subscriptions

Subscribers register a **topic pattern** rather than a fixed topic string.

| Wildcard | Matches | Example pattern |
|---|---|---|
| `*` | Any single segment | `work.*.project-x.*.createbranch` |
| `#` | Any remaining segments (suffix only) | `work.agency-abc.#` |

---

### 4. Fan-Out Delivery

Multiple independent subscribers can hold overlapping patterns. Each receives its own copy of every matching event. Publishers do not need to know who is listening.

---

### 5. At-Least-Once Delivery with Replay

PubSub guarantees at-least-once delivery. Unacknowledged events are retried. Subscribers must be idempotent and deduplicate by event ID.

Historical events are queryable by topic pattern, agencyID, and time range — enabling new services to bootstrap their state without a database snapshot.

---

### 6. CodeValdCross Integration

PubSub registers its HTTP routes with CodeValdCross via the standard 20-second heartbeat registrar. No Cross recompile is needed when routes are added.

---

## What CodeValdPubSub Does NOT Do

| Out of Scope | Reason |
|---|---|
| Exactly-once delivery | At-least-once is sufficient; subscribers are required to be idempotent |
| Ordered delivery across topics | Intra-topic ordering is preserved; cross-topic ordering is not guaranteed |
| Message transformation | PubSub routes events as published; no payload mutation |
| Authorization enforcement | Handled by CodeValdCross's policy layer (deferred until CodeValdOrg lands) |
| General-purpose byte payloads | Events are structured CodeVald lifecycle events; no arbitrary binary |
