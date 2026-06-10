# CodeValdPubSub — Active Bug Backlog

## Overview

Bugs in scope for CodeValdPubSub. Mirrors the `mvp.md` / `mvp_done.md` / `mvp-details/` layout used for feature work.

- **Fixed bugs**: see [`bugs_done.md`](bugs_done.md)
- **Per-bug detail**: see [`bug-details/`](bug-details/)
- **Master cross-service queue**: [`../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md`](../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md)

## Workflow

### Completion Process (MANDATORY)
1. Implement and validate (`go build ./...`, `go vet ./...`, `go test -race ./...`)
2. Move the bug row from this file to `bugs_done.md`
3. Update the detail file's Status header to `✅ Fixed (YYYY-MM-DD)` and cite the commit / branch
4. Strike-through + ✅ the entry on the master prioritization.md
5. Merge feature branch to main

### Status Legend
- 📋 **Open** — not yet started or in triage
- 🚀 **In Progress** — actively being worked
- ⏸️ **Blocked** — waiting on a dependency
- ✅ **Fixed** — moved to `bugs_done.md` (do not list here)

---

## Active Bugs

| Bug ID | Title | Severity | Status | Depends On |
|--------|-------|----------|--------|------------|
| [BUG-20260610-001](bug-details/BUG-20260610-001_events_after_fence_drops_all_events.md) | `GET /pubsub/{agency}/events?after=…` drops every event because stored events have no `timestamp` field | High | 📋 Open | — |

---

### BUG-20260610-001 — `GET /pubsub/{agency}/events?after=…` drops every event

**Severity:** High — every QA script and downstream consumer that fences on a baseline timestamp silently receives an empty event list, masking real flow activity.
**Status:** 📋 Open
**Detail:** [bug-details/BUG-20260610-001](bug-details/BUG-20260610-001_events_after_fence_drops_all_events.md)

Stored events have no populated `timestamp` field (every row serializes with an empty `timestamp` on the listing response). The `?after=` handler compares stored `timestamp` to the query value; with `timestamp == ""` or `null`, every row is excluded. Today's scenario 12 Work-1 trace looked empty (`0 events since baseline`) even though CodeValdWork logs proved publishes happened. Fix: persist `published_at` on every publish; canonicalise the read-side filter; surface the field on the GET response.
