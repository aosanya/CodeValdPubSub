# CodeValdPubSub — Fixed Bug Log

Mirrors the `mvp_done.md` layout. New entries prepended; oldest at the bottom.

| Bug ID | Title | Fixed | Commit / Branch |
|--------|-------|-------|------------------|
| [BUG-20260610-001](bug-details/BUG-20260610-001_events_after_fence_drops_all_events.md) | `GET /pubsub/{agency}/events?after=…` drops every event because stored events have no `timestamp` field | 2026-06-10 | main — `manager.ListEvents` prefers PublishedAt→CreatedAt and includes unstamped rows; `eventToProto` falls back to CreatedAt. Residual `?after=` query-key normalization filed as a separate Cross bug. |
