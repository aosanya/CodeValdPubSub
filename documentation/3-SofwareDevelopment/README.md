# 3 — Software Development

## Overview

This section tracks the MVP task list and per-task implementation details for CodeValdPubSub.

---

## Index

| Document | Description |
|---|---|
| [mvp.md](mvp.md) | Active task backlog — MVP tasks and status |
| [mvp_done.md](mvp_done.md) | Completed tasks |
| [mvp-details/](mvp-details/README.md) | Per-task implementation specifications |

---

## Workflow

### Completion Process (MANDATORY)
1. Implement and validate (`go build ./...`, `go vet ./...`, `go test -race ./...`)
2. Add row to `mvp_done.md`
3. Remove task from `mvp.md`
4. Mark dependency references as `~~PUBSUB-XXX~~ ✅`
5. Merge feature branch to main and delete it

### Branch Management
```bash
git checkout -b feature/PUBSUB-XXX_description
# implement + validate
git checkout main
git merge feature/PUBSUB-XXX_description --no-ff
git branch -d feature/PUBSUB-XXX_description
```

### Status Legend
- 📋 **Not Started** — ready to begin (dependencies met)
- 🚀 **In Progress** — currently being worked on
- ⏸️ **Blocked** — waiting on dependencies
- ✅ **Done** — complete
