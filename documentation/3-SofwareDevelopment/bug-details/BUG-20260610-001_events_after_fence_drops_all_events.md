# BUG-20260610-001 — `GET /pubsub/{agency}/events?after=…` drops every event because stored events have no `timestamp` field

**Status:** ✅ Fixed (2026-06-10) — `manager.ListEvents` filter prefers `PublishedAt` and falls back to `CreatedAt`; rows with neither timestamp are surfaced instead of silently dropped. `eventToProto` falls back to `CreatedAt` when `PublishedAt` is missing/unparseable so the response always carries a populated timestamp when storage has one. Unit tests cover all three branches.

**Residual (separate Cross bug):** the user-facing `?after=...` query key is rejected by Cross's proxy because `protojson.Unmarshal` returns "unknown field after" — the proto field is `after_timestamp`. `jq '.events | length'` on the resulting error envelope evaluates `null | length == 0`, which is how the empty-events symptom presents to QA. The Cross-side proxy needs to normalize URL query keys to canonical proto field names (mirror the case-insensitive snake_case match already used inside `wrapBodyIfNeeded`). File as a follow-up Cross bug; not in scope for this PubSub-owned ticket.

**Severity:** High — every QA script and downstream consumer that fences on a baseline timestamp silently receives an empty event list, masking real flow activity. Today's scenario 12 run looked like nothing fired even though the planner ran the full pipeline; only by removing the fence did the truth surface.
**Owner:** CodeValdPubSub
**Estimated effort:** ~0.5 day (persist a `timestamp` on every stored event; or, if it is persisted under a different field name, fix the read-side filter to read the canonical field).
**Source finding:** Session 2026-06-10, scenario [12 — utility-app-builder Planning Flow](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/) Work-1 verification, plus every `?after=` query in [scenario 12's work-*.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/).

## Problem

QA fences PubSub queries on a baseline timestamp so that events from earlier sessions don't pollute the result:

```bash
PUBSUB_BASELINE_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
curl ".../pubsub/{agency}/events?topic=task.assigned&after=${PUBSUB_BASELINE_TIME}"
```

The endpoint always returns `{"events": []}` even when, in the same window, CodeValdWork logs prove the publish happened:

```
codevaldwork-1 | 2026/06/10 11:34:47 codevaldwork: publish: topic="task.updated" agencyID="utility-app-builder" …
```

Removing the `?after=` fence returns the events. So either:

- PubSub stores events without a `timestamp` field at all (filter receives `null` on every row → drops every row).
- PubSub stores it under a different field name than the read-side filter looks up.

The empirical signature: the events endpoint with no fence returns 1208 events; the listing's `timestamp` field is empty (`"timestamp":"?"` in our serializer) for every row.

## Evidence

Query without fence — events come back:

```
$ curl '/pubsub/utility-app-builder/events' | python3 -c 'print(len(__import__("json").load(__import__("sys").stdin)["events"]))'
1208
```

Sample of the latest row — no `timestamp`:

```json
{
  "topic": "task.started",
  "payload": {"TaskID": "1c320ad8-…", "RunID": "ec071bc9-…", "AgentID": "36321ab7-…"}
  // no "timestamp" / "createdAt" / "publishedAt" / "ts"
}
```

Same query with fence:

```
$ curl '/pubsub/utility-app-builder/events?after=2026-06-10T11:33:00Z' | jq '.events | length'
0
```

Scenario 12's verification scripts ([work-01-task-create-assign.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-01-task-create-assign.md), [work-02-ai-run.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-02-ai-run.md), [work-03-task-complete.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-03-task-complete.md)) all use `?after=$BASELINE` and would have falsely reported PASS=0 / FAIL on every step today.

## Root cause

Hypothesis (needs confirmation by inspecting the storage write path): the publish handler persists events without populating a `timestamp` column / field on the entity, OR populates it under a name that the read-side filter doesn't read.

The `after` query handler compares stored `timestamp` to the query value; with `timestamp == ""` or `null`, every row is excluded. There is no fallback (e.g. compare against `createdAt`, or treat empty as "include").

## Fix plan

**Phase 1 — Confirm the canonical timestamp field.** Inspect `work_events` (or wherever PubSub persists routed events) and identify whether `timestamp` is missing entirely or stored under a different key. Use `aql` directly:

```
FOR e IN <pubsub_events_collection> LIMIT 5 RETURN e
```

**Phase 2 — Stamp every published event with `published_at` (or `timestamp`)** at write time, using the server clock. Backfill is optional — `WHERE timestamp IS NULL OR timestamp >= @after` works for historical rows.

**Phase 3 — Read-side filter:** the `?after=` handler must compare against the canonical field. If a row lacks the field, treat it as "include" (so old/unstamped rows still surface) and log a one-time warning.

**Phase 4 — Surface the field on the listing response.** Today the GET response serializes a row with no `timestamp` key at all; clients render it as `"?"`. Always include `timestamp` (and `published_at` if distinct) so QA scripts can sanity-check ordering.

## Verification

After fix, scenario 12 Work-1 must succeed without removing the fence:

```bash
ASSIGN_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
curl -X PUT "${BASE}/work/utility-app-builder/tasks/${T}/assignee/${A}" -d '{}'
sleep 2
curl "${BASE}/pubsub/utility-app-builder/events?topic=task.assigned&after=${ASSIGN_TIME}" \
  | jq '.events | length'
# must be ≥ 1, not 0
```

Every event row in the response must carry a populated `timestamp` (or `published_at`) field.

## Dependencies

- None known. Independent of the CodeValdWork / CodeValdAI changes filed today.
