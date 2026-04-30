// Package codevaldpubsub — pre-delivered schema definition.
//
// DefaultPubSubSchema returns the fixed [types.Schema] seeded on startup.
// The operation is idempotent — calling it multiple times with the same
// schema ID is safe.
//
// Entity types:
//   - Topic        — registered named channel (mutable, stored in pubsub_topics)
//   - Message      — immutable published event (append-only, stored in pubsub_messages)
//   - Subscription — a consumer's cursor on a topic (mutable, stored in pubsub_subscriptions)
//
// Graph topology:
//
//	Topic ──has_message──────► Message
//	Topic ──has_subscription──► Subscription
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
				UniqueKey:         []string{"name"},
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString},
					{Name: "created_at", Type: types.PropertyTypeString},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "has_message",
						Label:       "Messages",
						PathSegment: "messages",
						ToType:      "Message",
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
				Name:              "Message",
				DisplayName:       "Message",
				PathSegment:       "messages",
				EntityIDParam:     "messageId",
				StorageCollection: "pubsub_messages",
				Immutable:         true,
				Properties: []types.PropertyDefinition{
					{Name: "message_id", Type: types.PropertyTypeString, Required: true},
					{Name: "topic_name", Type: types.PropertyTypeString, Required: true},
					{Name: "payload", Type: types.PropertyTypeString},
					{Name: "attributes", Type: types.PropertyTypeString},
					{Name: "published_at", Type: types.PropertyTypeString, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					{
						Name:        "for_topic",
						Label:       "Topic",
						PathSegment: "topic",
						ToType:      "Topic",
						ToMany:      false,
						Inverse:     "has_message",
					},
				},
			},
			{
				Name:              "Subscription",
				DisplayName:       "Subscription",
				PathSegment:       "subscriptions",
				EntityIDParam:     "subscriptionId",
				StorageCollection: "pubsub_subscriptions",
				UniqueKey:         []string{"name"},
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "topic_name", Type: types.PropertyTypeString, Required: true},
					{Name: "cursor", Type: types.PropertyTypeString},
					{Name: "created_at", Type: types.PropertyTypeString},
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
