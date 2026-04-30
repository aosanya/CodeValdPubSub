# CodeValdPubSub — AI Agent Development Instructions

## Project Overview

**CodeValdPubSub** is a **Go library** that provides hierarchical pub/sub event recording for the CodeValdCortex enterprise multi-agent AI orchestration platform.

It gives every CodeVald service a shared, typed event bus. Services publish events on structured topics; the library routes, delivers, and records them for audit and replay.

**Core Concept**: Topics follow a dot-separated hierarchy: `<service>.<agencyID>.<projectName>.<taskName>.<event>`. For example, when CodeValdWork starts a task it publishes `work.<agencyID>.<projectName>.<taskName>.createbranch`. CodeValdPubSub records every event, enabling subscribers to react in real-time and operators to replay history.

---

## Library Architecture

### 1. Three-Interface Design

The library exposes three top-level interfaces:

```go
// Publisher — publishes a single event on a topic.
type Publisher interface {
    Publish(ctx context.Context, topic Topic, payload []byte) error
}

// Subscriber — subscribes to topics by exact match or glob pattern.
type Subscriber interface {
    Subscribe(ctx context.Context, pattern string, handler EventHandler) (Subscription, error)
    Unsubscribe(ctx context.Context, sub Subscription) error
}

// Recorder — persists events and supports replay/history queries.
type Recorder interface {
    Record(ctx context.Context, event Event) error
    Replay(ctx context.Context, pattern string, from time.Time) ([]Event, error)
    History(ctx context.Context, pattern string, limit int) ([]Event, error)
}

// EventHandler is the callback signature for subscribers.
type EventHandler func(ctx context.Context, event Event) error
```

### 2. Topic Naming Convention (MANDATORY)

Topics are dot-separated strings with a fixed segment structure:

```
<service>.<agencyID>.<projectName>.<taskName>.<event>
```

| Segment | Example | Notes |
|---|---|---|
| `service` | `work`, `git`, `agency`, `ai` | Originating CodeVald service (lowercase) |
| `agencyID` | `agency-abc-001` | UUID or slug identifying the agency |
| `projectName` | `my-project` | Project slug (lowercase, hyphenated) |
| `taskName` | `implement-auth` | Task slug (lowercase, hyphenated) |
| `event` | `createbranch`, `complete`, `failed` | Lifecycle event name (lowercase, no separators) |

**Wildcard patterns** (for subscriptions):
- `work.*.*.*.createbranch` — all branch-creation events across every agency/project/task
- `work.agency-abc-001.*.*.*` — all work events for a specific agency
- `*.*.*.*.*` — every event on the bus

### 3. Event Model

```go
// Event is the immutable record of a single published message.
type Event struct {
    ID          string    // UUID assigned at publish time
    Topic       Topic     // Parsed, validated topic
    Payload     []byte    // Raw bytes; callers own serialisation
    PublishedAt time.Time // UTC timestamp set by the library
    ServiceID   string    // Originating service identifier
}

// Topic is a validated, parsed topic string.
type Topic struct {
    Raw         string
    Service     string
    AgencyID    string
    ProjectName string
    TaskName    string
    EventName   string
}
```

### 4. Storage Backends

| Interface | Package | Purpose |
|---|---|---|
| `storage.Storer` | `codevaldpubsub/storage` | Persists events by topic, supports range/pattern queries |

**Filesystem (default)**:
```
{base_path}/events/{service}/{agency-id}/{yyyy-mm-dd}.jsonl
```

**ArangoDB** (custom `storage.Storer` in `storage/arangodb/`):

| Collection | Contents |
|---|---|
| `pubsub_events` | Every published event (keyed by event ID) |
| `pubsub_subscriptions` | Active subscription records |

> The caller constructs the desired `storage.Storer` and passes it to `NewBroker`. **CodeValdPubSub itself is backend-agnostic.**

### 5. Broker — Wiring Publisher, Subscriber, and Recorder Together

```go
// Broker is the single entry point that wires all three interfaces.
type Broker interface {
    Publisher
    Subscriber
    Recorder
}

// NewBroker constructs a Broker backed by the given storage.Storer.
func NewBroker(store storage.Storer) (Broker, error)
```

---

## Project Structure

```
/workspaces/CodeValdPubSub/
├── documentation/
│   ├── README.md
│   ├── 1-SoftwareRequirements/
│   │   ├── README.md
│   │   └── requirements.md
│   ├── 2-SoftwareDesignAndArchitecture/
│   │   ├── README.md
│   │   └── architecture.md
│   ├── 3-SofwareDevelopment/
│   │   ├── README.md
│   │   ├── mvp.md
│   │   └── mvp-details/
│   └── 4-QA/
│       └── README.md
├── .github/
│   ├── copilot-instructions.md
│   ├── instructions/
│   │   └── rules.instructions.md
│   ├── prompts/
│   └── workflows/
│       └── ci.yml
└── [Go module root]
    ├── go.mod
    ├── broker.go           # Broker interface + NewBroker constructor
    ├── publisher.go        # Publisher interface + implementation
    ├── subscriber.go       # Subscriber interface + Subscription type + implementation
    ├── recorder.go         # Recorder interface + implementation
    ├── errors.go           # ErrInvalidTopic, ErrSubscriptionNotFound, ErrStorageFull
    ├── models.go           # Event, Topic, Subscription (pure value types)
    ├── topic/
    │   └── parser.go       # Topic parsing, validation, glob pattern matching
    ├── storage/
    │   ├── storer.go       # storage.Storer interface
    │   ├── filesystem/     # JSONL file-per-day storage
    │   └── arangodb/       # ArangoDB storage.Storer implementation
    └── internal/
        └── router/         # In-process subscription routing + pattern matching
```

---

## Developer Workflows

### Build & Test Commands

```bash
# Run all tests with race detector
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Build check (library — verifies compilation, no binary produced)
go build ./...

# Static analysis
go vet ./...

# Format code
go fmt ./...

# Lint
golangci-lint run ./...
```

**There is no `make run`, no binary, no HTTP server.** This is a library — `go build ./...` only verifies it compiles cleanly.

### Task Management Workflow

```bash
# 1. Create feature branch from main
git checkout -b feature/PS-XXX_description

# 2. Implement changes

# 3. Build validation before merge
go build ./...           # Must succeed
go vet ./...             # Must show 0 issues
go test -v -race ./...   # Must pass
golangci-lint run ./...  # Must pass

# 4. Merge when complete
git checkout main
git merge feature/PS-XXX_description --no-ff
git branch -d feature/PS-XXX_description
```

---

## Technology Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.21+ | Matches CodeValdCortex; native concurrency |
| Storage (default) | Filesystem JSONL | Simple, portable, human-readable event log |
| Storage (optional) | ArangoDB via custom `storage.Storer` | Survives container restarts; supports pattern queries |
| Pattern matching | Glob via `internal/router` | Supports `*` wildcards per topic segment |

---

## Code Quality Rules

### Library-Specific Rules

- **No web framework dependencies** — no Gin, no HTTP handlers, no templ
- **No database driver in the core package** — ArangoDB storer lives in `storage/arangodb/`, never imported by root package
- **Interface-first** — callers depend on `Broker`, `Publisher`, `Subscriber`, `Recorder` interfaces, not concrete types
- **Exported API is minimal** — expose only what callers need; keep routing internals unexported
- **All public functions must have godoc comments**
- **Context propagation** — every public method takes `context.Context` as first argument

### Naming Conventions

- **Package name**: `codevaldpubsub` (root), `arangodb` (storage subpackage), `router` (internal)
- **Interfaces**: `Broker`, `Publisher`, `Subscriber`, `Recorder` — noun-only, no `I` prefix
- **Errors**: `Err` prefix — `ErrInvalidTopic`, `ErrSubscriptionNotFound`, `ErrStorageFull`
- **No abbreviations in exported names** — prefer `AgencyID` over `AgID`
- **Singular package names** — `storage`, not `storages`

### File Organisation

- **Max file size**: 500 lines (prefer smaller, focused files)
- **Max function length**: 50 lines (prefer 20-30)
- **One primary concern per file** — `broker.go`, `publisher.go`, `subscriber.go`, `recorder.go`, `errors.go`, `models.go`
- **Error types in `errors.go`** — never scatter sentinel errors across files
- **Value types in `models.go`** — `Event`, `Topic`, `Subscription`

### Anti-Patterns to Avoid

- ❌ **Publishing without a valid topic** — always parse and validate via `topic.Parse()`
- ❌ **Hardcoding storage backends in core package** — inject via `storage.Storer`
- ❌ **Silently dropping events** — every `Publish` must either record or return an error
- ❌ **Panicking in exported functions** — return structured errors
- ❌ **Ignoring context cancellation** — check `ctx.Err()` in subscriber dispatch loops
- ❌ **Coupling Recorder to a specific storage format** — serialisation lives in `storage/`, not `recorder.go`

---

## Integration with CodeVald Services

CodeVald services call CodeValdPubSub at these lifecycle points:

| Service Event | CodeValdPubSub Call | Topic |
|---|---|---|
| Work task created | `Publisher.Publish(...)` | `work.<agencyID>.<project>.<task>.created` |
| Work task branch created | `Publisher.Publish(...)` | `work.<agencyID>.<project>.<task>.createbranch` |
| Work task completed | `Publisher.Publish(...)` | `work.<agencyID>.<project>.<task>.complete` |
| Git commit pushed | `Publisher.Publish(...)` | `git.<agencyID>.<project>.<task>.commit` |
| Agency created | `Publisher.Publish(...)` | `agency.<agencyID>._._. created` |
| Replay audit log | `Recorder.Replay(...)` | — |
| UI activity feed | `Recorder.History(...)` | — |

---

## Documentation References

- `documentation/1-SoftwareRequirements/requirements.md` — functional requirements, NFR, resolved open questions
- `documentation/2-SoftwareDesignAndArchitecture/architecture.md` — design decisions, storage backends, topic schema, draft interfaces
- `documentation/3-SofwareDevelopment/mvp.md` — MVP task list and status
- `documentation/3-SofwareDevelopment/mvp-details/` — per-topic task specifications
- `documentation/4-QA/README.md` — testing strategy and QA standards

---

## When in Doubt

1. **Check documentation first**: requirements and architecture are the source of truth
2. **Interface before implementation**: define the interface, write tests against it, then implement
3. **Inject dependencies**: storage is always caller-provided
4. **Write tests for every exported function** — aim for >80% coverage; use table-driven tests
5. **Topic validation is always the first step** — call `topic.Parse()` before any publish or subscribe operation
