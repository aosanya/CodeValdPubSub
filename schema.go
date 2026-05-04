// Package codevaldpubsub — pre-delivered schema definition.
//
// DefaultPubSubSchema returns the fixed [types.Schema] seeded on startup.
// The operation is idempotent — calling it multiple times with the same
// schema ID is safe.
//
// Entity types:
//   - Topic        — registered named channel (mutable, stored in pubsub_topics)
//   - Event        — immutable recorded event (append-only, stored in pubsub_events)
//   - Subscription — a service's registration for a topic pattern (mutable, stored in pubsub_subscriptions)
//
// Graph topology:
//
//	Event ──for_topic──────────► Topic
//	Subscription ──subscribes_to──► Topic
package codevaldpubsub

import "github.com/aosanya/CodeValdSharedLib/types"

// DefaultPubSubSchema returns the pre-delivered [types.Schema].
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
				UniqueKey:         []string{"pattern"},
				Properties: []types.PropertyDefinition{
					{Name: "pattern", Type: types.PropertyTypeString, Required: true},
					{Name: "domain", Type: types.PropertyTypeString},
					{Name: "action", Type: types.PropertyTypeString},
					{Name: "source_service", Type: types.PropertyTypeString},
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
				Immutable:         true,
				Properties: []types.PropertyDefinition{
					{Name: "topic", Type: types.PropertyTypeString, Required: true},
					{Name: "domain", Type: types.PropertyTypeString},
					{Name: "agency_id", Type: types.PropertyTypeString},
					{Name: "action", Type: types.PropertyTypeString},
					{Name: "payload", Type: types.PropertyTypeString},
					{Name: "source_service", Type: types.PropertyTypeString},
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
					{Name: "subscriber_id", Type: types.PropertyTypeString, Required: true},
					{Name: "subscriber_service", Type: types.PropertyTypeString},
					{Name: "topic_pattern", Type: types.PropertyTypeString, Required: true},
					{Name: "status", Type: types.PropertyTypeString},
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
						Inverse:     "has_subscription",
					},
				},
			},
		},
	}
}
