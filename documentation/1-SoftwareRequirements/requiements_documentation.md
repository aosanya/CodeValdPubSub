# CodeValdPubSub — Topic Catalog

This document is the authoritative reference for all event topics published by CodeVald platform services. Every entry includes the topic template, the publishing service, the trigger condition, the payload type, and example subscription patterns.

---

## Topic Naming Convention

```
<service>.<agencyID>.<entity-segment-1>.<entity-segment-2>….<action>
```

| Segment | Position | Description |
|---|---|---|
| `service` | 1 | Originating service (`work`, `git`, `agency`, `comm`, `dt`) |
| `agencyID` | 2 | The owning agency — always present as the second segment |
| `entity-segments` | 3…N-1 | Zero or more resource identifiers narrowing the scope (project name, task name, repo ID, branch ID, etc.) |
| `action` | N (last) | What happened (`created`, `completed`, `merged`, `createbranch`, `conflict.detected`, …) |

Entity segments use the resource's human-readable name where available (project name, task name) or its entitygraph ID where a name is unavailable.

---

## Design Resolutions

### DR-001: AgencyID in topic position 2

AgencyID is always the second segment. This makes the pattern `*.<agencyID>.#` a natural way to subscribe to all events for a single agency across all services. The alternative (agencyID in payload only) would require subscribers to filter in application code rather than at the routing layer.

### DR-002: Entity segments use names, not IDs, where stable

Project names and task names are used in topic segments because they are stable identifiers in the CodeVald data model. Repo IDs and branch IDs are used where no stable name exists. If a name changes, previously published events retain the old name in their topic — the topic is an immutable fact about when the event occurred.

### DR-003: Action is always the last segment

The action is the terminal segment. This allows `<prefix>.#` subscriptions to receive all event types under a given entity hierarchy, and `<prefix>.<action>` subscriptions to receive only a specific event type.

### DR-004: Cross-service workflow events are published by the initiating service

When CodeValdWork starts a task, it publishes `work.<agencyID>.<projectName>.<taskName>.createbranch` — not CodeValdGit. The event records the intent from the work-management perspective. CodeValdGit subscribes to this topic and performs the branch creation. This preserves publisher autonomy: CodeValdWork does not need to call CodeValdGit directly.

---

## 1. CodeValdWork Topics

### `work.<agencyID>.<projectName>.<taskName>.created`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | A Task entity is successfully created |
| **Payload** | `TaskCreatedPayload{TaskID, Priority}` |

**Subscription examples:**
```
work.agency-abc.#                          # all work events for agency-abc
work.agency-abc.project-x.#               # all work events for project-x in agency-abc
work.agency-abc.project-x.task-001.created
work.*.*.*.created                         # all task creation events across all agencies
```

---

### `work.<agencyID>.<projectName>.<taskName>.updated`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | A non-status mutable field on a Task changes |
| **Payload** | `TaskUpdatedPayload{TaskID, ChangedFields[]string}` |

---

### `work.<agencyID>.<projectName>.<taskName>.status.changed`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | Every successful task status transition |
| **Payload** | `TaskStatusChangedPayload{TaskID, From TaskStatus, To TaskStatus}` |

**Note:** Also fires for terminal statuses. `work.*.*.*.completed` fires additionally (see below).

---

### `work.<agencyID>.<projectName>.<taskName>.completed`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | Task reaches a terminal status (completed, failed, cancelled) |
| **Payload** | `TaskCompletedPayload{TaskID, TerminalStatus, CompletedAt string}` |

Published **in addition to** `work.*.*.*.status.changed`.

**Subscription examples:**
```
work.agency-abc.*.*.completed              # all completed tasks in agency-abc
work.*.*.*.completed                       # all completed tasks platform-wide
```

---

### `work.<agencyID>.<projectName>.<taskName>.assigned`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | An `assigned_to` edge is created or replaced on a Task |
| **Payload** | `TaskAssignedPayload{TaskID, AgentID}` |

---

### `work.<agencyID>.<projectName>.<taskName>.createbranch`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | Task transitions to `in_progress`; signals that a Git branch should be created |
| **Payload** | `TaskBranchRequestPayload{TaskID, ProjectName, TaskName, BranchName string}` |

**This is a workflow event.** CodeValdGit subscribes to `work.<agencyID>.*.*.createbranch` and creates the corresponding task branch in the agency's Git repository. CodeValdWork does not call CodeValdGit directly.

**Subscription examples:**
```
work.agency-abc.*.*.createbranch          # all branch creation requests for agency-abc
work.agency-abc.project-x.*.createbranch  # branch creation requests for project-x
```

---

### `work.<agencyID>.relationship.created`

| Field | Value |
|---|---|
| **Publisher** | CodeValdWork |
| **Trigger** | A whitelisted graph edge is created between Work entities |
| **Payload** | `RelationshipCreatedPayload{FromID, ToID, Label string}` |

---

## 2. CodeValdGit Topics

### `git.<agencyID>.<repoID>.created`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | `InitRepo` completes successfully |
| **Payload** | `RepoCreatedPayload{RepoID, Name string}` |

---

### `git.<agencyID>.<repoID>.imported`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | Async `ImportRepo` job completes successfully |
| **Payload** | `RepoImportedPayload{JobID, RepoID string}` |

---

### `git.<agencyID>.<repoID>.import.failed`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | Async `ImportRepo` job fails |
| **Payload** | `RepoImportFailedPayload{JobID, ErrorMessage string}` |

---

### `git.<agencyID>.<repoID>.import.cancelled`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | Async `ImportRepo` job is cancelled |
| **Payload** | `RepoImportCancelledPayload{JobID string}` |

---

### `git.<agencyID>.<repoID>.<branchID>.fetched`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | Async `FetchBranch` job completes successfully |
| **Payload** | `BranchFetchedPayload{JobID, BranchID, RepoID string}` |

---

### `git.<agencyID>.<repoID>.<branchID>.merged`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | A branch is successfully merged into the repository default branch |
| **Payload** | `BranchMergedPayload{BranchID, RepoID string}` |

**Subscription examples:**
```
git.agency-abc.*.*.merged                  # all branch merges in agency-abc
git.*.*.*.merged                           # all branch merges platform-wide
```

---

### `git.<agencyID>.<repoID>.<branchID>.conflict.detected`

| Field | Value |
|---|---|
| **Publisher** | CodeValdGit |
| **Trigger** | `MergeBranch` encounters a conflict that cannot be auto-resolved |
| **Payload** | `MergeConflictPayload{BranchID string, ConflictingFiles []string}` |

---

## 3. CodeValdAgency Topics

### `agency.<agencyID>.created`

| Field | Value |
|---|---|
| **Publisher** | CodeValdAgency |
| **Trigger** | A new Agency entity is created |
| **Payload** | `AgencyCreatedPayload{AgencyID, Name string}` |

---

### `agency.<agencyID>.published`

| Field | Value |
|---|---|
| **Publisher** | CodeValdAgency |
| **Trigger** | An `AgencyPublication` (versioned snapshot) is created from a draft |
| **Payload** | `AgencyPublishedPayload{AgencyID, PublicationID, Version string}` |

---

### `agency.<agencyID>.promoted`

| Field | Value |
|---|---|
| **Publisher** | CodeValdAgency |
| **Trigger** | A draft is promoted to the live Agency entity |
| **Payload** | `AgencyPromotedPayload{AgencyID, DraftID string}` |

---

## 4. Subscription Pattern Reference

| Pattern | Receives |
|---|---|
| `*.agency-abc.#` | All events for agency-abc across all services |
| `work.agency-abc.#` | All CodeValdWork events for agency-abc |
| `work.agency-abc.project-x.#` | All CodeValdWork events for project-x |
| `work.agency-abc.project-x.task-001.#` | All events for a single task |
| `work.*.*.*.createbranch` | Branch creation requests across all agencies |
| `work.*.*.*.completed` | All task completions across all agencies |
| `git.agency-abc.*.*.merged` | All branch merges in agency-abc |
| `git.*.*.*.conflict.detected` | All merge conflicts platform-wide |
| `agency.*.created` | All new agency creations |
| `#` | All events (operator / monitoring use) |
