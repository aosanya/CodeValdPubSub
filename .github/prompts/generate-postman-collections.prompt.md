---
agent: agent
---

# Generate Postman Collections for CodeValdPubSub

This prompt guides generation of Postman collections for the CodeValdPubSub gRPC service.

## Objective

Produce a Postman collection (or grpcurl command reference) that covers all PubSub gRPC endpoints. CodeValdPubSub uses **Protocol Buffers v3** and exposes a `PubSubService` via gRPC.

## Steps

### 1. Identify the Service Definition

The proto definition lives at:
```
proto/codevaldpubsub/v1/service.proto
```

Generated Go stubs are in:
```
gen/go/codevaldpubsub/v1/
```

### 2. Key RPC Methods to Cover

| Method | Description |
|---|---|
| `Publish` | Publish an event to a topic |
| `Subscribe` | Open a server-streaming subscription for a topic pattern |
| `Unsubscribe` | Cancel an active subscription |
| `History` | Retrieve past events for a topic pattern |

### 3. Topic Format

All topics must follow the five-segment format validated by `topic.Parse`:
```
service.agencyID.projectName.taskName.event
```
Example: `work.agency-001.my-project.task-42.createbranch`

Wildcards (`*`) are allowed in Subscribe/History patterns but **never** in Publish topics.

### 4. grpcurl Reference

```bash
# Publish an event
grpcurl -plaintext -d '{
  "topic": "work.agency-001.proj-a.task-1.createbranch",
  "payload": "<base64-encoded-payload>"
}' localhost:50055 codevaldpubsub.v1.PubSubService/Publish

# Subscribe (server streaming)
grpcurl -plaintext -d '{
  "pattern": "work.agency-001.*.*.*"
}' localhost:50055 codevaldpubsub.v1.PubSubService/Subscribe

# History query
grpcurl -plaintext -d '{
  "pattern": "work.agency-001.*.*.*",
  "limit": 50
}' localhost:50055 codevaldpubsub.v1.PubSubService/History
```

### 5. Environment Variables for Postman

| Variable | Value |
|---|---|
| `base_url` | `localhost:50055` |
| `agency_id` | `agency-123` |

### 6. Collection Organisation

Structure the Postman collection as:
```
CodeValdPubSub/
├── Publishing/
│   ├── Publish work event
│   └── Publish git event
├── Subscriptions/
│   ├── Subscribe all agency events
│   └── Subscribe task-specific events
└── History/
    ├── Retrieve recent events
    └── Retrieve events by topic pattern
```

## Output Format

Provide:
1. A Postman collection JSON (v2.1 schema)
2. A grpcurl command reference for each RPC
3. Example payloads covering happy-path and error cases
