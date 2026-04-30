---
agent: agent
---

# Debug a CodeValdPubSub Issue

## How to Use This Prompt

When you encounter a bug in CodeValdPubSub, describe the failing behaviour and use the guidelines below to add targeted debug logging, isolate the cause, and clean up before merging.

## Common Failure Scenarios

### Scenario 1: Event Not Delivered to Subscriber
**Symptom**: `Publish` returns no error, but the subscriber handler is never called
**Cause**: Topic pattern mismatch in the router, or subscriber registered after publish
**Check**: Inspect `internal/router/` — verify pattern matching logic and subscription registration order

### Scenario 2: `Publish` Returns Storage Error
**Symptom**: `Publish` fails with an ArangoDB error before dispatching to subscribers
**Cause**: ArangoDB collection missing, credentials wrong, or `EnsureMessageIndexes` not called
**Check**: Confirm `pubsubadb.NewBackend` succeeded and `EnsureMessageIndexes` ran on startup

### Scenario 3: Duplicate Event Delivery
**Symptom**: Subscriber handler called multiple times for a single publish
**Cause**: Multiple matching subscriptions, or subscriber registered more than once
**Check**: Inspect subscription map — ensure `Unsubscribe` is called on teardown

### Scenario 4: Topic Validation Rejected Unexpectedly
**Symptom**: `topic.Parse` returns `ErrInvalidTopic` for a seemingly valid topic
**Cause**: Topic segment contains uppercase, spaces, or wrong number of dot-separated parts
**Check**: Verify exactly 5 dot-separated lowercase segments: `service.agencyID.project.task.event`

### Scenario 5: Context Cancellation Not Respected
**Symptom**: Subscriber dispatch loop hangs after context is cancelled
**Cause**: Missing `ctx.Err()` check in goroutine dispatch loop
**Check**: Add `ctx.Err()` check at top of each dispatch iteration

## Debug Print Guidelines

### Prefix Format
All debug prints MUST be prefixed with: `[TASK-ID]`

### Go
```go
log.Printf("[PS-XXX] Function called: %s with args: %+v", functionName, args)
log.Printf("[PS-XXX] State before: %+v", state)
log.Printf("[PS-XXX] Error in operation: %v", err)
```
### Strategic Placement

Add debug prints at:

1. **Function Entry Points**
   - Log function name and key parameters
   - Example: `log.Printf("[PS-XXX] Publish called: topic=%s", topic.Raw)`

2. **State Changes**
   - Before and after critical state modifications
   - Example: `log.Printf("[PS-XXX] Subscriber count before dispatch: %d", len(subs))`

3. **Conditional Branches**
   - Log which branch is taken and why
   - Example: `log.Printf("[PS-XXX] Pattern matched=%v topic=%s pattern=%s", matched, topic.Raw, pattern)`

4. **Loop Iterations** (for subscriber dispatch loop)
   - Log iteration count and key variables
   - Example: `log.Printf("[PS-XXX] Dispatching to subscriber %d/%d", i, total)`

5. **Error Handling**
   - Log errors with context before returning
   - Example: `log.Printf("[PS-XXX] Publish failed: %v", err)`

6. **Return Statements** (for complex functions)
   - Log what is being returned
   - Example: `log.Printf("[PS-XXX] Publish result: recorded=%v dispatched=%d", recorded, dispatched)`

### What NOT to Debug

Avoid adding debug prints to:
- Simple getters/setters
- Trivial utility functions
- Hot paths called thousands of times per operation
- Already well-instrumented code

### Debug Print Structure

Use descriptive messages that answer:
1. **WHERE**: Which function/block is executing
2. **WHAT**: What operation is happening
3. **VALUES**: Relevant variable values
4. **CONTEXT**: Why this matters (optional)

**Good Example:**
```go
log.Printf("[PS-XXX] Publish: topic=%s matchedSubs=%d storageOK=%v",
    topic.Raw, len(subs), storageErr == nil)
```

**Bad Example:**
```go
fmt.Println("here") // No context, no task ID, not helpful
```

### Cleanup Instructions

Always add a comment above debug blocks:
```go
// TODO: Remove debug log for PS-XXX after issue is resolved
log.Printf("[PS-XXX] Debug info here")
```

## Execution Steps

1. **Identify Task ID** from branch or context
2. **Analyse the failing code path** in `pubsub.go`, `manager_impl.go`, or `internal/router/`
3. **Select strategic points** where debug prints will be most valuable
4. **Add debug prints** with proper format and task ID prefix
5. **Run tests** and filter output: `go test ./... 2>&1 | grep PS-XXX`
6. **Explain placement** briefly — why each print was added

## Output Format

After adding debug prints, provide:

```markdown
### Debug Prints Added for [TASK-ID]

**File**: `path/to/file.go`

**Locations**:
1. Line XX: Function entry — logs parameters
2. Line YY: State change — logs before/after values
3. Line ZZ: Conditional check — logs decision logic

**Usage**: Run `go test -v ./... 2>&1 | grep PS-XXX` to see output.
```

## Remember

- **Always** use task ID prefix
- **Be strategic** - don't over-instrument
- **Be descriptive** - logs should tell a story
- **Be consistent** - use same format throughout
- **Be removable** - add TODO comments for cleanup
