# Problem Definition

## The Problem

The CodeVald platform is a collection of independent microservices — CodeValdWork, CodeValdGit, CodeValdAgency, CodeValdComm, CodeValdDT — each owning a slice of platform state. These services must react to each other's lifecycle events:

- When CodeValdWork starts a task, CodeValdGit must create a branch.
- When CodeValdGit merges a branch, CodeValdWork must advance the task status.
- When CodeValdAgency publishes a new agency, CodeValdWork must provision its project structures.

Before CodeValdPubSub, the platform handled this with **ad-hoc direct calls routed through CodeValdCross**:

| Problem | Consequence |
|---|---|
| No durable event record | If the receiving service was down, the event was lost — no replay, no recovery |
| Tight coupling | Each service had to know which other services cared about its events |
| No audit trail | No way to reconstruct the sequence of cross-service state changes that led to a given outcome |
| No fan-out | An event could only be consumed by one downstream handler; adding a second consumer required code changes |
| AgencyID scattered | Each service carried its own ad-hoc `CrossPublisher` stub that only logged events |

---

## The Solution

**CodeValdPubSub** is the durable, hierarchical event bus for the platform:

- **Durable**: Every event is written to ArangoDB before being routed. A subscriber that is temporarily down will receive missed events on reconnect.
- **Hierarchical topics**: Topics embed the agency ID and entity identifiers — `work.<agencyID>.<projectName>.<taskName>.createbranch` — so subscribers can filter at any level of granularity.
- **Fan-out**: Any number of subscribers can independently receive the same event. Adding a new consumer requires no change to the publisher.
- **Replay**: Historical events can be queried by topic pattern, agency, and time range. A new service can bootstrap its state by replaying past events.
- **Audit trail**: The event log is the authoritative record of cross-service state transitions.

---

## What PubSub Is Not

- It is **not** a general-purpose message queue. Events are structured CodeVald lifecycle events, not arbitrary byte payloads.
- It is **not** a replacement for the `eventbus.Publisher` interface used for intra-service logging. PubSub is the durable cross-service routing layer that eventually backs that interface.
- It is **not** a command bus. Publishers emit facts ("this happened"); subscribers decide what to do about them.
