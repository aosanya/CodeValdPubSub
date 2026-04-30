# CodeValdPubSub — Documentation

## Overview

**CodeValdPubSub** is the durable event recorder and pub/sub routing layer for the CodeVald platform.

Every platform service — CodeValdWork, CodeValdGit, CodeValdAgency, and others — publishes lifecycle events to CodeValdPubSub. PubSub records each event durably in ArangoDB and routes it to any registered subscriber.

PubSub is not a generic message broker. It is an **opinionated, agency-scoped event bus** built on the same entitygraph infrastructure as the rest of the platform. Topics embed the agency ID and entity hierarchy, so subscribers can express fine-grained interest: "all task completions in agency X" or "all branch merges for project Y".

---

## Documentation Index

| Document | Description |
|---|---|
| [1-SoftwareRequirements/](1-SoftwareRequirements/README.md) | What the service must do — scope, FR, NFR, topic catalog |
| [2-SoftwareDesignAndArchitecture/](2-SoftwareDesignAndArchitecture/README.md) | Design decisions, topic naming, routing, storage, delivery guarantees |
| [3-SofwareDevelopment/](3-SofwareDevelopment/README.md) | MVP task list and implementation details |
| [4-QA/](4-QA/README.md) | Testing strategy and acceptance criteria |

### Key Files

| File | Description |
|---|---|
| [1-SoftwareRequirements/requirements.md](1-SoftwareRequirements/requirements.md) | Functional requirements (FR-001–FR-008), NFR |
| [1-SoftwareRequirements/requiements_documentation.md](1-SoftwareRequirements/requiements_documentation.md) | Topic catalog — all platform event topics, payloads, and subscription patterns |
| [2-SoftwareDesignAndArchitecture/architecture.md](2-SoftwareDesignAndArchitecture/architecture.md) | Core design decisions, topic format, routing engine, storage schema |
| [3-SofwareDevelopment/mvp.md](3-SofwareDevelopment/mvp.md) | MVP task list and status |

---

## Quick Summary

- **Language**: Go
- **Transport**: gRPC + HTTP proxy via CodeValdCross
- **Storage**: ArangoDB (entitygraph — events and subscriptions stored as named entities)
- **Role**: Durable pub/sub recorder; all platform services are publishers
- **Topic format**: `<service>.<agencyID>.<entity-segments…>.<action>` (dot-separated)
- **Subscription**: Wildcard `*` per segment; multi-segment `#` suffix
- **Delivery guarantee**: At-least-once; subscribers dedup by event ID
