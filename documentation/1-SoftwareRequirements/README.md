# 1 — Software Requirements

## Overview

This section captures everything **what** CodeValdPubSub must do and **why** — without prescribing how.

---

## Index

| Document | Description |
|---|---|
| [requirements.md](requirements.md) | Functional requirements (FR-001–FR-008), non-functional requirements, scope, and open questions |
| [requiements_documentation.md](requiements_documentation.md) | Topic catalog — all platform event topics, payloads, and subscription pattern examples |
| [introduction/problem-definition.md](introduction/problem-definition.md) | Problem statement and motivation |
| [introduction/high-level-features.md](introduction/high-level-features.md) | High-level capability summary |
| [introduction/stakeholders.md](introduction/stakeholders.md) | Publishers, subscribers, and operator roles |

---

## Summary

CodeValdPubSub is the **durable pub/sub event recorder** for the CodeVald platform. All platform services publish lifecycle events to it; subscribers receive those events at-least-once and may replay history.

### Core Requirements at a Glance

| FR | Requirement |
|---|---|
| FR-001 | Durable event recording — every published event is written to ArangoDB before routing |
| FR-002 | Hierarchical topic routing — `<service>.<agencyID>.<entity-segments…>.<action>` |
| FR-003 | Pattern subscriptions — wildcard `*` per segment, `#` multi-segment suffix |
| FR-004 | Fan-out delivery — multiple independent subscribers per topic pattern |
| FR-005 | At-least-once delivery — unacknowledged events are retried |
| FR-006 | Event replay — query historical events by topic pattern, agency, and time range |
| FR-007 | CodeValdCross integration — routes and gRPC methods registered via heartbeat |
| FR-008 | Agency isolation — subscribers receive only events within their agency scope |
