// Package codevaldpubsub is the system-wide event bus for CodeVald services.
//
// Services publish events on topic strings of the form:
//
//	<domain>.<agencyId>.<subjectKey>.<action>
//
// Examples:
//
//	work.agency123.MVP-001.createbranch
//	git.agency123.main.merged
//	agency.agency123.-.drafted
//
// # Core types
//
//   - [Manager] — service-level operations: topics, events, subscriptions.
//   - [CrossPublisher] — optional hook to forward events to CodeValdCross.
package codevaldpubsub

import "context"

// Manager is the top-level interface for CodeValdPubSub operations.
// One Manager is shared process-wide; all methods are safe for concurrent use.
type Manager interface {
	// ── Topics ────────────────────────────────────────────────────────────────

	// RegisterTopic creates a new Topic entity for agencyID.
	// Returns ErrTopicAlreadyRegistered if the pattern already exists.
	RegisterTopic(ctx context.Context, agencyID string, req RegisterTopicRequest) (Topic, error)

	// GetTopic retrieves a Topic by its entity ID.
	// Returns ErrTopicNotFound if no topic with that ID exists.
	GetTopic(ctx context.Context, agencyID, topicID string) (Topic, error)

	// GetTopicByPattern retrieves a Topic by its pattern string.
	// Returns ErrTopicNotFound if no topic with that pattern exists.
	GetTopicByPattern(ctx context.Context, agencyID, pattern string) (Topic, error)

	// ListTopics returns all Topic entities for agencyID matching filter.
	ListTopics(ctx context.Context, agencyID string, filter TopicFilter) ([]Topic, error)

	// DeleteTopic removes a Topic entity.
	// Returns ErrTopicNotFound if no topic with that ID exists.
	DeleteTopic(ctx context.Context, agencyID, topicID string) error

	// ── Events ────────────────────────────────────────────────────────────────

	// RecordEvent creates an immutable Event entity.
	// The matching Topic must exist; returns ErrTopicNotFound otherwise.
	RecordEvent(ctx context.Context, agencyID string, req RecordEventRequest) (Event, error)

	// GetEvent retrieves an Event by its entity ID.
	// Returns ErrEventNotFound if no event with that ID exists.
	GetEvent(ctx context.Context, agencyID, eventID string) (Event, error)

	// ListEvents returns Event entities for agencyID matching filter.
	ListEvents(ctx context.Context, agencyID string, filter EventFilter) ([]Event, error)

	// ── Subscriptions ─────────────────────────────────────────────────────────

	// Subscribe registers a new Subscription entity for agencyID.
	// Returns ErrSubscriptionNotFound if the topic pattern has no matching Topic.
	Subscribe(ctx context.Context, agencyID string, req SubscribeRequest) (Subscription, error)

	// GetSubscription retrieves a Subscription by its entity ID.
	// Returns ErrSubscriptionNotFound if no subscription with that ID exists.
	GetSubscription(ctx context.Context, agencyID, subscriptionID string) (Subscription, error)

	// ListSubscriptions returns all Subscription entities for agencyID matching filter.
	ListSubscriptions(ctx context.Context, agencyID string, filter SubscriptionFilter) ([]Subscription, error)

	// UpdateSubscription mutates the status of an existing Subscription.
	// Valid status values: "active", "paused", "cancelled".
	// Returns ErrSubscriptionNotFound if no subscription with that ID exists.
	UpdateSubscription(ctx context.Context, agencyID, subscriptionID string, req UpdateSubscriptionRequest) (Subscription, error)

	// Unsubscribe removes a Subscription entity.
	// Returns ErrSubscriptionNotFound if no subscription with that ID exists.
	Unsubscribe(ctx context.Context, agencyID, subscriptionID string) error
}

// CrossPublisher is an optional hook called after each RecordEvent.
// Implementations forward the event notification to CodeValdCross for
// service-mesh fan-out. A nil CrossPublisher silently skips forwarding.
type CrossPublisher interface {
	NotifyEvent(ctx context.Context, agencyID, topic, eventID string) error
}
