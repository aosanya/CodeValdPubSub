// Package codevaldpubsub provides the pub/sub event recorder for the CodeVald
// platform. Value types used by PubSubManager and its callers are defined here.
//
// Three entity types mirror [DefaultPubSubSchema]:
//   - Topic        → "pubsub_topics"        (mutable, unique by pattern)
//   - Event        → "pubsub_events"        (immutable, append-only)
//   - Subscription → "pubsub_subscriptions" (mutable, lifecycle: active/paused/cancelled)
//
// Edges live in "pubsub_relationships":
//
//	Topic ──has_event────────► Event
//	Topic ──has_subscription──► Subscription
package codevaldpubsub

// Topic is a registered event topic pattern stored in "pubsub_topics".
// Each Topic is unique by its Pattern field. Services register Topics on
// startup so that consumers can discover what events are available.
type Topic struct {
	// ID is the unique entitygraph identifier for this Topic entity.
	ID string `json:"id"`

	// Pattern is the wildcard topic template, e.g. "work.*.*.*.createbranch".
	// Segments: <domain>.<agencyId>.<projectName>.<entityName>.<action>.
	Pattern string `json:"pattern"`

	// Domain is the service namespace extracted from the pattern: "work", "git",
	// "agency", etc.
	Domain string `json:"domain"`

	// Action is the terminal segment of the pattern, e.g. "createbranch".
	Action string `json:"action"`

	// SourceService is the service that publishes events on this topic.
	SourceService string `json:"source_service,omitempty"`

	// Description is a human-readable explanation of when this topic fires.
	Description string `json:"description,omitempty"`

	// CreatedAt is the ISO 8601 timestamp when the Topic was registered.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the ISO 8601 timestamp when the Topic was last modified.
	UpdatedAt string `json:"updated_at"`
}

// Event is an immutable record of a single published event stored in
// "pubsub_events". Events are append-only — never mutated after creation.
// The fully-resolved topic string encodes all routing information.
type Event struct {
	// ID is the unique entitygraph identifier for this Event entity.
	ID string `json:"id"`

	// Topic is the fully-resolved topic string for this event, e.g.
	// "work.agency123.myproject.MVP-001.createbranch".
	Topic string `json:"topic"`

	// Domain is the first topic segment: "work", "git", "agency", etc.
	Domain string `json:"domain"`

	// AgencyID is the second topic segment — the universal scoping key.
	AgencyID string `json:"agency_id"`

	// Action is the terminal topic segment, e.g. "createbranch".
	Action string `json:"action"`

	// Payload is the JSON-encoded event payload. Schema is action-specific
	// and defined by the publishing service.
	Payload string `json:"payload,omitempty"`

	// SourceService is the service that published this event.
	SourceService string `json:"source_service,omitempty"`

	// PublishedAt is the RFC 3339 timestamp at which the event was published.
	PublishedAt string `json:"published_at"`

	// CreatedAt is the ISO 8601 timestamp when this entity was persisted.
	CreatedAt string `json:"created_at"`
}

// Subscription is a service's registered interest in a topic pattern,
// stored in "pubsub_subscriptions". Linked to its Topic via a subscribes_to
// edge. Status lifecycle: "active" → "paused" → "cancelled".
type Subscription struct {
	// ID is the unique entitygraph identifier for this Subscription entity.
	ID string `json:"id"`

	// TopicID is the entitygraph ID of the linked Topic, resolved from the
	// subscribes_to edge. Empty when the subscription was created without a
	// TopicID.
	TopicID string `json:"topic_id,omitempty"`

	// SubscriberID is a stable external identifier for the subscriber,
	// e.g. a service name or agent ID.
	SubscriberID string `json:"subscriber_id"`

	// SubscriberService is the service or component holding this subscription.
	SubscriberService string `json:"subscriber_service,omitempty"`

	// TopicPattern is the wildcard pattern this subscription covers,
	// e.g. "work.*.*.*.createbranch" or "work.agency123.*.*.*".
	TopicPattern string `json:"topic_pattern"`

	// Status is the lifecycle state: "active", "paused", or "cancelled".
	Status string `json:"status"`

	// CreatedAt is the ISO 8601 timestamp when the Subscription was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the ISO 8601 timestamp when the Subscription was last modified.
	UpdatedAt string `json:"updated_at"`
}

// ── Request / filter types ────────────────────────────────────────────────────

// RegisterTopicRequest carries the parameters for [PubSubManager.RegisterTopic].
type RegisterTopicRequest struct {
	// Pattern is the wildcard topic template. Required.
	Pattern string `json:"pattern"`

	// Domain is the service namespace. Required.
	Domain string `json:"domain"`

	// Action is the terminal segment of the pattern. Required.
	Action string `json:"action"`

	// SourceService is the service that publishes on this topic.
	SourceService string `json:"source_service,omitempty"`

	// Description is a human-readable explanation of when this topic fires.
	Description string `json:"description,omitempty"`
}

// TopicFilter constrains the result set returned by [PubSubManager.ListTopics].
type TopicFilter struct {
	// Domain filters to topics with the given domain. Empty means all domains.
	Domain string `json:"domain,omitempty"`

	// Action filters to topics with the given action. Empty means all actions.
	Action string `json:"action,omitempty"`

	// Limit caps the number of topics returned. 0 means no limit.
	Limit int `json:"limit,omitempty"`
}

// RecordEventRequest carries the parameters for [PubSubManager.RecordEvent].
type RecordEventRequest struct {
	// Topic is the fully-resolved topic string. Required.
	Topic string `json:"topic"`

	// Domain is the first topic segment. Required.
	Domain string `json:"domain"`

	// AgencyID is the second topic segment. Required.
	AgencyID string `json:"agency_id"`

	// Action is the terminal topic segment. Required.
	Action string `json:"action"`

	// Payload is the JSON-encoded event payload.
	Payload string `json:"payload,omitempty"`

	// SourceService is the publishing service name.
	SourceService string `json:"source_service,omitempty"`

	// PublishedAt is the RFC 3339 timestamp of publication.
	// Defaults to the current UTC time when empty.
	PublishedAt string `json:"published_at,omitempty"`
}

// EventFilter constrains the result set returned by [PubSubManager.ListEvents].
type EventFilter struct {
	// TopicID restricts to events linked to this Topic entity ID.
	TopicID string `json:"topic_id,omitempty"`

	// Domain restricts to events with this domain segment.
	Domain string `json:"domain,omitempty"`

	// AgencyID restricts to events with this agency ID segment.
	AgencyID string `json:"agency_id,omitempty"`

	// Action restricts to events with this action segment.
	Action string `json:"action,omitempty"`

	// Limit caps the number of events returned. 0 means no limit.
	Limit int `json:"limit,omitempty"`
}

// SubscribeRequest carries the parameters for [PubSubManager.Subscribe].
type SubscribeRequest struct {
	// TopicID is the entitygraph ID of the Topic to link. Optional.
	TopicID string `json:"topic_id,omitempty"`

	// SubscriberID is a stable identifier for the subscriber. Required.
	SubscriberID string `json:"subscriber_id"`

	// SubscriberService is the service or component holding this subscription.
	SubscriberService string `json:"subscriber_service,omitempty"`

	// TopicPattern is the wildcard pattern this subscription covers. Required.
	TopicPattern string `json:"topic_pattern"`
}

// SubscriptionFilter constrains the result set returned by
// [PubSubManager.ListSubscriptions].
type SubscriptionFilter struct {
	// SubscriberID restricts to subscriptions held by this subscriber.
	SubscriberID string `json:"subscriber_id,omitempty"`

	// SubscriberService restricts to subscriptions for this service.
	SubscriberService string `json:"subscriber_service,omitempty"`

	// Status restricts to subscriptions with this lifecycle state.
	// Valid values: "active", "paused", "cancelled". Empty means all.
	Status string `json:"status,omitempty"`

	// Limit caps the number of subscriptions returned. 0 means no limit.
	Limit int `json:"limit,omitempty"`
}

// UpdateSubscriptionRequest carries the mutable fields for
// [PubSubManager.UpdateSubscription].
type UpdateSubscriptionRequest struct {
	// Status is the new lifecycle state.
	// Valid values: "active", "paused", "cancelled". Required.
	Status string `json:"status"`
}
