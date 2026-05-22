**CodeValdPubSub** is the durable event bus that ties the CodeVald platform together. Instead of direct point-to-point calls between services, every lifecycle event — task started, branch merged, agency created — is written to ArangoDB before routing, surviving restarts and enabling full replay. Topics are hierarchical and agency-scoped (`work.<agencyID>.<task>.completed`), and subscribers register patterns with `*` and `#` wildcards. Multiple independent consumers can fan-out from the same event without the publisher knowing who's listening.

Part of the **CodeVald** platform — the backbone that makes loosely coupled, event-driven AI agent coordination possible.

GitHub: https://github.com/aosanya/CodeValdPubSub
