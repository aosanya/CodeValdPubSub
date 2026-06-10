# CodeValdPubSub — Fixed Bug Log

Mirrors the `mvp_done.md` layout. New entries prepended; oldest at the bottom.

| Bug ID | Title | Fixed | Commit / Branch |
|--------|-------|-------|------------------|
| [BUG-20260610-001](bug-details/BUG-20260610-001_events_after_fence_drops_all_events.md) | `GET /pubsub/{agency}/events?after=…` drops every event because stored events have no `timestamp` field | 2026-06-10 | main — root cause was the proto field being named `after_timestamp` while the public query key is `?after=`; Cross's proxy could not map the two and returned `BAD_REQUEST` (which QA scripts read as `events: 0`). Renamed `QueryEventsRequest.after_timestamp` → `after`, regenerated stubs, plumbed `req.After` through the server. Storage and `manager.ListEvents` (PublishedAt→CreatedAt fallback) were already correct. |
