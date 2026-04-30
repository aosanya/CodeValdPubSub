// Package codevaldpubsub — pre-delivered schema definition.
//
// This file exposes [DefaultPubSubSchema], which returns the fixed [types.Schema]
// for CodeValdPubSub. cmd/main.go seeds this schema idempotently on startup via
// PubSubSchemaManager.SetSchema.
//
// CodeValdPubSub is the system-wide event recorder. Every service publishes events
// on hierarchical topic strings of the form:
//
//	<domain>.<agencyId>.<projectName>.<entityName>.<action>
//
// Examples:
//
//	work.agency123.myproject.MVP-001.createbranch
//	work.agency123.myproject.MVP-001.updatestatus
//	git.agency123.myrepo.main.merged
//	agency.agency123.-.-.drafted
//
// The schema declares three TypeDefinitions:
//   - Topic        — registered topic pattern, e.g. "work.*.*.*.createbranch" (mutable)
//   - Event        — immutable record of a single published event (immutable)
//   - Subscription — a service's registered interest in a topic pattern (mutable)
//
// Graph topology:
//
//	Topic ──has_event────────► Event
//	Topic ──has_subscription──► Subscription
//
// Storage:
//   - Topic        → "pubsub_topics"        document collection
//   - Event        → "pubsub_events"        document collection (immutable, append-only)
//   - Subscription → "pubsub_subscriptions" document collection
//   - All edges    → "pubsub_relationships" edge collection
package codevaldpubsub

import "github.com/aosanya/CodeValdSharedLib/types"

// DefaultPubSubSchema returns the pre-delivered [types.Schema] seeded by
// cmd/main.go on startup. The operation is idempotent — calling it multiple
// times with the same schema ID is safe.
func DefaultPubSubSchema() types.Schema {
	return types.Schema{
		ID:      "pubsub-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []types.TypeDefinition{
			{
				Name:              "Topic",
				DisplayName:       "Topic",
				PathSegment:       "topics",
				EntityIDParam:     "topicId",
				StorageCollection: "pubsub_topics",
				// pattern is the natural unique key — each topic pattern is registered once.
				UniqueKey: []string{"pattern"},
				Properties: []types.PropertyDefinition{
					// pattern is the wildcard topic template, e.g. "work.*.*.*.createbranch".
					// Segments: <domain>.<agencyId>.<projectName>.<entityName>.<action>.
					// Use "*" for segments that vary per event.
					{Name: "pattern", Type: types.PropertyTypeString, Required: true},
					// domain is the service namespace: "work", "git", "agency", etc.
					{Name: "domain", Type: types.PropertyTypeString, Required: true},
					// action is the terminal segment, e.g. "createbranch", "updatestatus", "merged".
					{Name: "action", Type: types.PropertyTypeString, Required: true},
					// source_service is the service that publishes events on this topic.
					{Name: "source_service", Type: types.PropertyTypeString},
					// description is a human-readable explanation of when this topic fires.
					{Name: "description", Type: types.PropertyTypeString},
					{Name: "created_at", Type: types.PropertyTypeString},
					{Name: "updated_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "has_event",
						Label:       "Events",
						PathSegment: "events",
						ToType:      "Event",
						ToMany:      true,
						Inverse:     "for_topic",
					},
					{
						Name:        "has_subscription",
						Label:       "Subscriptions",
						PathSegment: "subscriptions",
						ToType:      "Subscription",
						ToMany:      true,
						Inverse:     "subscribes_to",
					},
				},
			},
			{
				Name:              "Event",
				DisplayName:       "Event",
				PathSegment:       "events",
				EntityIDParam:     "eventId",
				StorageCollection: "pubsub_events",
				// Events are append-only records — never mutated after creation.
				Immutable: true,
				Properties: []types.PropertyDefinition{
					// topic is the fully-resolved topic string for this event, e.g.:
					// "work.agency123.myproject.MVP-001.createbranch"
					{Name: "topic", Type: types.PropertyTypeString, Required: true},
					// domain is the first topic segment: "work", "git", "agency", etc.
					{Name: "domain", Type: types.PropertyTypeString, Required: true},
					// agency_id is the second topic segment — the only universal scoping key.
					{Name: "agency_id", Type: types.PropertyTypeString, Required: true},
					// action is the terminal topic segment, e.g. "createbranch", "updatestatus".
					// The middle segments are domain-specific and are not decomposed here;
					// consumers can parse them from the full topic string.
					{Name: "action", Type: types.PropertyTypeString, Required: true},
					// payload is the JSON-encoded event payload. Schema is action-specific
					// and defined by the publishing service, not PubSub.
					{Name: "payload", Type: types.PropertyTypeString},
					// source_service is the service that published this event.
					{Name: "source_service", Type: types.PropertyTypeString},
					// published_at is the RFC 3339 timestamp at which the event was published.
					{Name: "published_at", Type: types.PropertyTypeString, Required: true},
					{Name: "created_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "for_topic",
						Label:       "Topic",
						PathSegment: "topic",
						ToType:      "Topic",
						ToMany:      false,
						Inverse:     "has_event",
					},
				},
			},
			{
				Name:              "Subscription",
				DisplayName:       "Subscription",
				PathSegment:       "subscriptions",
				EntityIDParam:     "subscriptionId",
				StorageCollection: "pubsub_subscriptions",
				Properties: []types.PropertyDefinition{
					// subscriber_id is a stable external identifier for the subscriber,
					// e.g. a service name or agent ID. Unique per topic_pattern.
					{Name: "subscriber_id", Type: types.PropertyTypeString, Required: true},
					// subscriber_service is the service or component holding this subscription.
					{Name: "subscriber_service", Type: types.PropertyTypeString},
					// topic_pattern is the wildcard pattern this subscription covers,
					// e.g. "work.*.*.*.createbranch" or "work.agency123.*.*.*".
					{Name: "topic_pattern", Type: types.PropertyTypeString, Required: true},
					// status is the lifecycle state: "active", "paused", "cancelled".
					{Name: "status", Type: types.PropertyTypeString, Required: true},
					{Name: "created_at", Type: types.PropertyTypeString},
					{Name: "updated_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "subscribes_to",
						Label:       "Topic",
						PathSegment: "topic",
						ToType:      "Topic",
						ToMany:      false,
						Required:    true,
						Inverse:     "has_subscription",
					},
				},
			},
		},
	}
}
