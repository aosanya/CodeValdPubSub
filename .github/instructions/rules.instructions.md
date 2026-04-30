---
applyTo: '**'
---

# CodeValdPubSub — Code Structure Rules

## Library Design Principles

CodeValdPubSub is a **Go library** — not an application. These rules reflect that:

- **No HTTP handlers, no web framework, no templating engine**
- **No `main` package** — the root package is the library entry point
- **Callers inject dependencies** — storage backends are never hardcoded
- **Exported API surface is minimal** — expose only what consumers need

---

## Interface-First Design

**Always define interfaces before concrete types.**

```go
// ✅ CORRECT — interface in root package, consumed by CodeValdWork/CodeValdGit/etc.
type Publisher interface {
    Publish(ctx context.Context, topic Topic, payload []byte) error
}

// ❌ WRONG — leaking a concrete type to callers
type ArangoPublisher struct {
    col driver.Collection
}
```

**File layout — one primary concern per file:**

```
broker.go      → Broker interface + NewBroker constructor
publisher.go   → Publisher interface + implementation
subscriber.go  → Subscriber interface + Subscription type + implementation
recorder.go    → Recorder interface + implementation
errors.go      → all exported error types
models.go      → Event, Topic, Subscription (pure value types, no methods)
```

---

## Topic Validation Rules

**ALL publish and subscribe calls must use a validated Topic.**

```go
// ✅ CORRECT — topic parsed and validated before publish
t, err := topic.Parse("work.agency-001.my-project.my-task.createbranch")
if err != nil {
    return err
}
broker.Publish(ctx, t, payload)

// ❌ WRONG — raw string passed directly (bypasses validation)
broker.PublishRaw(ctx, "work.agency-001.my-project.my-task.createbranch", payload)
```

**Topic segment rules (enforced by `topic.Parse`):**
1. Exactly five dot-separated segments: `service.agencyID.projectName.taskName.event`
2. All segments lowercase, hyphen-separated words only (`[a-z0-9-]+`)
3. Wildcards (`*`) allowed in subscription patterns but **never** in published topics
4. Empty segments are rejected

---

## Publish / Subscribe Contract

```go
// ✅ CORRECT — subscriber registered before any publish
sub, err := broker.Subscribe(ctx, "work.*.*.*.createbranch", func(ctx context.Context, e Event) error {
    // handle event
    return nil
})
defer broker.Unsubscribe(ctx, sub)

// ❌ WRONG — subscriber is never unsubscribed (leaks goroutine/memory)
broker.Subscribe(ctx, "work.*.*.*.*", handler)
```

**`Publish` must**:
1. Parse and validate the topic
2. Record the event via `Recorder.Record` before dispatching to subscribers
3. Dispatch to all matching subscribers (pattern match via `internal/router`)
4. Return an error if recording fails — never silently drop events

---

## Storage Backend Rules

The `storage.Storer` interface is the injection point. The caller constructs the desired backend and passes it to `NewBroker`. The root package never imports any storage driver.

```go
// ✅ CORRECT — backend injected by caller; root package stays backend-agnostic
store, _ := arangodb.NewStore(arangodb.Config{
    Endpoint:   "http://localhost:8529",
    Collection: "pubsub_events",
})
broker, _ := codevaldpubsub.NewBroker(store)

// ❌ WRONG — hardcoded backend inside library
func NewBroker() Broker {
    store := arangodb.NewStore(...)  // hardcoded!
    // ...
}
```

- `storage/filesystem/` and `storage/arangodb/` are **public** packages — callers can import them directly
- `internal/router/` holds the subscription routing logic — not importable outside this module
- **Never import ArangoDB drivers from the root package**

---

## Error Handling Rules

**All exported errors must be typed and structured:**

```go
// errors.go

// ErrInvalidTopic is returned when a topic string fails validation.
type ErrInvalidTopic struct {
    Raw    string
    Reason string
}

func (e ErrInvalidTopic) Error() string {
    return fmt.Sprintf("invalid topic %q: %s", e.Raw, e.Reason)
}

var ErrSubscriptionNotFound = errors.New("subscription not found")
var ErrStorageFull = errors.New("event storage capacity exceeded")
```

- **Never use `log.Fatal`** in library code — return errors to caller
- **Never panic** in exported functions
- **Wrap errors with context**: `fmt.Errorf("Publish %s: %w", topic.Raw, err)`

---

## Context Rules

**Every exported method must accept `context.Context` as the first argument.**

```go
// ✅ CORRECT
func (b *broker) Publish(ctx context.Context, topic Topic, payload []byte) error

// ❌ WRONG
func (b *broker) Publish(topic Topic, payload []byte) error
```

Respect context cancellation in subscriber dispatch loops and storage write operations.

---

## Godoc Rules

**Every exported type, function, interface, and method must have a godoc comment.**

```go
// Publish records the event to storage and dispatches it to all matching subscribers.
// The topic must be a validated Topic produced by topic.Parse — raw strings are rejected.
// Returns ErrInvalidTopic if the topic fails validation, or a storage error if recording fails.
func (b *broker) Publish(ctx context.Context, t Topic, payload []byte) error {
```

- **Package comment** on the primary file of every package
- **Examples** in `_test.go` files for non-obvious API usage patterns

---

## File Size and Complexity Limits

- **Max file size**: 500 lines (hard limit)
- **Max function length**: 50 lines (prefer 20-30)
- **One primary concern per file**
- **Split `storage/arangodb/` by concern** when it grows (e.g., `events.go`, `subscriptions.go`)

**Example of compliant file breakdown:**
```
✅ CORRECT:
storage/arangodb/
├── events.go         # Event insert + range query (~250 lines)
├── subscriptions.go  # Subscription records (~150 lines)
└── config.go         # Connection + collection setup (~100 lines)

❌ WRONG:
storage/arangodb/
└── storage.go        # Everything in one 600-line file
```

---

## Concurrency Rules

- **`Recorder.History` / `Recorder.Replay`** must be **safe to call concurrently**
- **`Publisher.Publish`** is serialised per storage write; subscriber dispatch runs concurrently via goroutines
- Subscriber `EventHandler` callbacks must not block — use goroutines for long work inside handlers
- Avoid shared mutable state in `Broker` implementations beyond the in-process subscription map (protected by `sync.RWMutex`)

---

## Naming Conventions

```go
// ✅ CORRECT — singular package names, noun-only interfaces, Err prefix for errors
package codevaldpubsub

type Broker interface{}
type Publisher interface{}
type Subscriber interface{}
type Recorder interface{}
var ErrSubscriptionNotFound = errors.New("subscription not found")
type ErrInvalidTopic struct{}

// ❌ WRONG
package codevaldpubsubs         // plural
type IBroker interface{}        // I prefix
var subscriptionNotFoundError   // unexported sentinel exposed via behaviour
```

---

## Task Management and Workflow

### Branch Management (MANDATORY)

```bash
# Create feature branch from main
git checkout -b feature/PS-XXX_description

# Implement and validate
go build ./...           # must succeed
go test -v -race ./...   # must pass
go vet ./...             # must show 0 issues
golangci-lint run ./...  # must pass

# Merge when complete
git checkout main
git merge feature/PS-XXX_description --no-ff
git branch -d feature/PS-XXX_description
```

### Pre-Development Checklist

Before adding new code:
1. ✅ Is this type already defined in `models.go` or `errors.go`?
2. ✅ Am I adding logic to the right layer (root package vs `storage/arangodb/` vs `internal/router/`)?
3. ✅ Does this function accept `context.Context` as its first argument?
4. ✅ Will the file exceed 500 lines after this change?
5. ✅ Am I injecting storage instead of hardcoding it?
6. ✅ Does every new exported symbol have a godoc comment?
7. ✅ Are topics validated via `topic.Parse` before use?

### Code Review Requirements

Every PR must verify:
- [ ] No hardcoded storage backends in root package
- [ ] All exported symbols have godoc comments
- [ ] Context propagated through all public calls
- [ ] Errors are typed (`ErrInvalidTopic`, not raw strings) for public errors
- [ ] No files exceeding 500 lines
- [ ] Tests added for all new exported functions
- [ ] `go vet ./...` shows 0 issues
- [ ] `go test -race ./...` passes
- [ ] Topics always validated via `topic.Parse` before publish/subscribe
