---
agent: agent
---

# Go File Modularization Prompt

You are a Go refactoring expert that helps split large files into smaller, focused packages while following Go best practices and clean architecture principles.

## When to Use This Prompt

- Go file exceeds 400 lines
- File contains multiple unrelated concerns
- Repository structure violates `.github/instructions/rules.instructions.md`
- File has poor separation of concerns
- Multiple types/structs with different responsibilities

## Go Package Organization Strategy

### Standard Project Structure (CodeValdPubSub)

```
codevaldpubsub/           ← root package (library entry point)
├── broker.go             # Broker interface + NewBroker constructor
├── publisher.go          # Publisher interface + implementation
├── subscriber.go         # Subscriber interface + Subscription type
├── recorder.go           # Recorder interface + implementation
├── errors.go             # Exported error types
├── models.go             # Event, Topic, Subscription value types
├── topic/
│   └── parser.go         # Topic parsing, validation, glob pattern matching
├── storage/
│   ├── storer.go         # storage.Storer interface
│   ├── filesystem/       # JSONL file-per-day storage
│   └── arangodb/         # ArangoDB storage.Storer implementation
│       ├── events.go     # Event insert + range queries
│       ├── subscriptions.go # Subscription records
│       └── config.go     # Connection + collection setup
└── internal/
    └── router/           # In-process subscription routing + pattern matching
```

### File Size Limits (ENFORCED)

- **Any file**: Max 500 lines (hard limit)
- **Functions**: Max 50 lines (prefer 20-30)
- **Split `storage/arangodb/` by concern** when it grows beyond 500 lines

## Refactoring Strategies

### Strategy 1: Split by Domain (Recommended)

**Before:**
```
internal/handlers/handler.go (800 lines)
├── User handlers
├── Order handlers
├── Product handlers
└── Payment handlers
```

**After:**
```
internal/interfaces/http/
├── user_handler.go      (150 lines)
├── order_handler.go     (180 lines)
├── product_handler.go   (140 lines)
└── payment_handler.go   (160 lines)
```

### Strategy 2: Split by Responsibility

**Before:**
```
internal/repository/repository.go (600 lines)
├── Event CRUD
├── Subscription CRUD
├── Statistics queries
└── Pattern matching
```

**After:**
```
internal/infrastructure/persistence/
├── event_repository.go       (120 lines) - Event CRUD
├── subscription_repository.go (130 lines) - Subscription CRUD
├── stats_service.go          (150 lines) - Statistics (separate service!)
└── query_builder.go          (100 lines) - Query utilities
```

### Strategy 3: Extract Helpers

**Before:**
```go
// big_service.go (500 lines)
func (s *Service) MainOperation() { ... }
func (s *Service) helperMethod1() { ... }
func (s *Service) helperMethod2() { ... }
func (s *Service) helperMethod3() { ... }
// ... many helper methods
```

**After:**
```go
// main_service.go (200 lines)
func (s *Service) MainOperation() {
    data := s.helper.Process()
    // ...
}

// helper.go (180 lines)
type Helper struct { ... }
func (h *Helper) Process() { ... }
func (h *Helper) Validate() { ... }
```

## Architectural Rules (MANDATORY)

### 1. No Duplicate Types

❌ **WRONG:**
```go
// package broker
type TopicSegment string

// package router
type TopicSegment string  // DUPLICATE!
```

✅ **CORRECT:**
```go
// models.go (root package)
type Topic struct {
    Raw         string
    Service     string
    AgencyID    string
    ProjectName string
    TaskName    string
    EventName   string
}

// Other packages import from root or topic/
import "codevaldpubsub/topic"
```

### 2. Clear Package Boundaries

✅ **Good Package Structure:**
```
internal/
├── router/              # Subscription routing, NO storage logic
└── (future internal pkgs)
storage/
├── storer.go            # storage.Storer interface only
├── filesystem/          # Filesystem impl, NO routing logic
└── arangodb/            # ArangoDB impl, NO routing logic
topic/
└── parser.go            # Topic parsing only, NO broker logic
```

### 3. Dependency Direction

```
root package (broker/publisher/subscriber/recorder)
    ↓
topic/ (parsing + validation)
    ↓
storage/ (persistence interface + implementations)
    ↓
internal/router/ (routing internals)
```

**Rules:**
- `internal/router` never imports storage packages
- `storage/arangodb` never imports `internal/router`
- Root package never imports ArangoDB drivers directly

### 4. Repository Pattern

❌ **WRONG - Business logic in storage:**
```go
func (s *ArangoStore) RecordAndNotify(e Event) error {
    // Storage + notification = BAD
    s.col.Insert(e)
    s.notifySubscribers(e)  // This is broker logic!
}
```

✅ **CORRECT - Storage only persists:**
```go
// Storage
func (s *ArangoStore) Record(ctx context.Context, e Event) error {
    _, err := s.col.CreateDocument(ctx, e)
    return err
}

// Broker (orchestration)
func (b *broker) Publish(ctx context.Context, t Topic, payload []byte) error {
    e := newEvent(t, payload)
    if err := b.store.Record(ctx, e); err != nil {
        return err
    }
    return b.router.Dispatch(ctx, e)
}
```

## Refactoring Process

### Step 1: Analyze Current File

```bash
# Check file size
wc -l broker.go

# Identify responsibilities
grep "^func " broker.go

# Check for duplicate types
grep -r "type Topic" .
```

### Step 2: Plan Module Structure

Create a refactoring plan:
```markdown
## Refactoring Plan: broker.go (800 lines)

**Target structure:**
- publisher.go        (180 lines)
- subscriber.go       (200 lines)
- recorder.go         (150 lines)
- broker.go           (80 lines)  ← wiring only

**Dependencies:**
- All use: storage.Storer, internal/router
- Common types: Event, Topic, Subscription (stay in models.go)

**Shared types to extract:**
- Topic → already in models.go / topic/parser.go
```

### Step 3: Create New Files

**File naming conventions:**
- Use snake_case: `event_store.go`, not `EventStore.go`
- Group by domain: `event_*.go`, `subscription_*.go`
- Be specific: `arangodb_event_store.go`, not `store.go`

**Package naming:**
- Use singular: `package storage`, not `package storages`
- Be descriptive: `package router`, not `package route`

### Step 4: Move Code

**Order of operations:**
1. Create new files with package declarations
2. Move type definitions
3. Move functions (with receivers)
4. Update imports
5. Run tests
6. Delete old file

### Step 5: Handle Shared Dependencies

**Extract shared types:**
```go
// Before: Topic defined in multiple packages

// After: models.go (root package)
package codevaldpubsub

type Topic struct {
    Raw         string
    Service     string
    AgencyID    string
    ProjectName string
    TaskName    string
    EventName   string
}
```

### Step 6: Update Original File

**CRITICAL:** The original file must be updated to prevent breaking existing imports.

**Option 1: Convert to Re-exporter (Recommended)**

```go
// broker.go (REPLACE ENTIRE CONTENT — thin wiring layer)
package codevaldpubsub

// Broker wires Publisher, Subscriber, and Recorder over a shared storage.Storer.
type Broker interface {
    Publisher
    Subscriber
    Recorder
}

// NewBroker constructs a Broker backed by the given store.
func NewBroker(store storage.Storer) (Broker, error) {
    r := router.New()
    return &broker{
        pub: newPublisher(store, r),
        sub: newSubscriber(r),
        rec: newRecorder(store),
    }, nil
}
```

### Step 7: Update Dependent Files

Update files that import the original package after any structural change.

## Testing After Refactoring

```bash
# Ensure tests still pass
go test ./...

# Check for circular dependencies
go build ./...

# Run linter
golangci-lint run

# Check import cycles
go list -f '{{.ImportPath}} {{.Imports}}' ./... | grep cycle
```

## Checklist

**Before refactoring:**
- [ ] File exceeds size limit (>400 lines)
- [ ] Identified distinct responsibilities
- [ ] Checked for duplicate types across packages
- [ ] Planned new package structure
- [ ] Reviewed architectural rules

**During refactoring:**
- [ ] Following dependency direction rules
- [ ] No duplicate types
- [ ] Clear package boundaries
- [ ] Consistent naming (snake_case files, singular packages)
- [ ] Each file has single responsibility

**After refactoring:**
- [ ] All tests pass
- [ ] No circular dependencies
- [ ] Files within size limits
- [ ] Imports are clean
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
- [ ] **Original file updated** (re-exporter or thin wiring layer)
- [ ] **Dependent files identified** and migration plan created
- [ ] **Backward compatibility** maintained (if needed)

## Output Format

```markdown
### Refactoring Summary

**Original file**: `broker.go` (800 lines)

**New structure**:
```
publisher.go        (180 lines)
subscriber.go       (200 lines)
recorder.go         (150 lines)
broker.go           (80 lines)
```

**Shared types extracted**:
- `models.go` — Event, Topic, Subscription

**Dependencies**:
- publisher depends on: storage.Storer, internal/router
- subscriber depends on: internal/router
- recorder depends on: storage.Storer

**Breaking changes**: None (backward compatible — Broker interface unchanged)
```
