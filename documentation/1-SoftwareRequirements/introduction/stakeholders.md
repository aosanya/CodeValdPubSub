# Stakeholders

## Publishers

Every platform service that emits lifecycle events is a publisher.

| Service | Published Topics (prefix) | When |
|---|---|---|
| **CodeValdWork** | `work.<agencyID>.<projectName>.<taskName>.*` | Task lifecycle: created, started, assigned, completed, failed, cancelled |
| **CodeValdGit** | `git.<agencyID>.<repoID>.*` | Repository and branch lifecycle: created, merged, conflict |
| **CodeValdAgency** | `agency.<agencyID>.*` | Agency lifecycle: created, published, promoted |
| **CodeValdComm** | `comm.<agencyID>.*` | Communication events (planned) |
| **CodeValdDT** | `dt.<agencyID>.*` | Data table events (planned) |

Publishers call `PubSubManager.Publish(ctx, Event)`. The PubSub service records the event durably and routes it to matching subscribers before returning.

---

## Subscribers

| Consumer | Subscriptions | Purpose |
|---|---|---|
| **CodeValdGit** | `work.<agencyID>.*.*.createbranch` | Creates a task branch when CodeValdWork starts a task |
| **CodeValdWork** | `git.<agencyID>.<repoID>.*.merged` | Advances task status when its branch is merged |
| **CodeValdAI** | `work.<agencyID>.*.*.completed` | Picks up completed tasks for AI post-processing |
| **Platform operators** | `*.#` (all events) | Monitoring, audit, debugging |

Subscribers register a topic pattern. PubSub delivers matching events at-least-once. Subscribers acknowledge receipt; unacknowledged events are retried.

---

## Operators

| Role | Interest |
|---|---|
| **Platform operators** | Event replay for incident reconstruction; retention policy management |
| **Agency admins** | Per-agency event audit (`*.<agencyID>.#`) |
| **AI agents (indirect)** | Agents trigger events via their host services; they do not publish to PubSub directly |

---

## Library Maintainers

CodeValdPubSub is maintained as part of the CodeVald platform alongside CodeValdWork, CodeValdGit, and CodeValdSharedLib. It follows the same development conventions:
- Go module at `github.com/aosanya/CodeValdPubSub`
- gRPC service registered with CodeValdCross via heartbeat registrar
- ArangoDB entitygraph for durable storage
