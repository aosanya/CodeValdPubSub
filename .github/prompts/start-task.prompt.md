---
agent: agent
---

# Start New Task

> ⚠️ **Before starting a new task**, run `CodeValdPubSub/.github/prompts/finish-task.prompt.md` to ensure any in-progress task is properly completed and merged first.

Follow the **mandatory task startup process** for CodeValdPubSub tasks:

## Task Startup Process (MANDATORY)

1. **Select the next task**
   - Check `documentation/3-SofwareDevelopment/mvp.md` for the task list and current status
   - Check `documentation/3-SofwareDevelopment/mvp-details/` for detailed specs per topic
   - Check `documentation/1-SoftwareRequirements/requirements.md` for unimplemented functional requirements
   - Prefer foundational tasks (e.g., `topic.Parse`, `Broker`, core interfaces) before dependent ones

2. **Read the specification**
   - Re-read the relevant FR(s) in `documentation/1-SoftwareRequirements/requirements.md`
   - Re-read the corresponding section in `documentation/2-SoftwareDesignAndArchitecture/architecture.md`
   - Read the task spec in `documentation/3-SofwareDevelopment/mvp-details/{topic-file}.md`
   - Understand how the task fits into the three-interface design (`Publisher`, `Subscriber`, `Recorder`)
   - Note topic validation requirements — `topic.Parse` must be called before every publish/subscribe

3. **Create feature branch from `main`**
   ```bash
   cd /workspaces/CodeValdPubSub
   git checkout main
   git pull origin main
   git checkout -b feature/PS-XXX_description
   ```
   Branch naming: `feature/PS-XXX_description` (lowercase with underscores)

4. **Read project guidelines**
   - Review `.github/instructions/rules.instructions.md`
   - Key rules: interface-first, context propagation, no hardcoded storage, topic validation, godoc on all exports

5. **Create a todo list**
   - Break the task into actionable steps
   - Use the manage_todo_list tool to track progress
   - Mark items in-progress and completed as you go

## Pre-Implementation Checklist

Before starting:
- [ ] Relevant FRs and architecture sections re-read
- [ ] Feature branch created: `feature/PS-XXX_description`
- [ ] Existing files checked — no duplicate types in `models.go` or `errors.go`
- [ ] Understood which file(s) to modify (`broker.go`, `publisher.go`, `subscriber.go`, `recorder.go`, `errors.go`, `models.go`, `topic/parser.go`, `storage/arangodb/`, `internal/router/`)
- [ ] Todo list created for this task

## Development Standards

- **No hardcoded storage** — inject via `storage.Storer`
- **Every exported symbol** must have a godoc comment
- **Every exported method** takes `context.Context` as the first argument
- **Topics must be validated** via `topic.Parse` before every `Publish` or `Subscribe` call
- **Errors** must be typed (`ErrInvalidTopic`, `ErrSubscriptionNotFound`, etc.) — not raw `errors.New` strings for public errors
- **Never silently drop events** — if `Recorder.Record` fails, `Publish` must return the error

## Git Workflow

```bash
# Create feature branch
git checkout -b feature/PS-XXX_description

# Regular commits during development
git add .
git commit -m "PS-XXX: Descriptive message"

# Build validation before merge
go build ./...           # must succeed
go test -v -race ./...   # must pass
go vet ./...             # must show 0 issues
golangci-lint run ./...  # must pass

# Merge when complete
git checkout main
git merge feature/PS-XXX_description --no-ff
git branch -d feature/PS-XXX_description
```

## Success Criteria

- ✅ Relevant FR(s) and architecture doc reviewed
- ✅ Feature branch created from `main`
- ✅ Todo list created with implementation steps
- ✅ Ready to implement following library design rules
