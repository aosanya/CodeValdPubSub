// Package codevaldpubsub manages pub/sub topics, events, and subscriptions
// for the CodeVald platform. Use [NewManager] to obtain an implementation.
package codevaldpubsub

import "context"

// Manager is the primary API surface for pub/sub operations.
type Manager interface {
	// Topics
	RegisterTopic(ctx context.Context, agencyID string, req RegisterTopicRequest) (Topic, error)
	// RegisterTopics upserts the full topic set for a producer service. It is
	// idempotent on producesHash: if PubSub already processed this exact hash
	// for (agencyID, sourceService) no DB writes occur.
	RegisterTopics(ctx context.Context, agencyID, sourceService, producesHash string, topics []RegisterTopicRequest) error
	GetTopic(ctx context.Context, agencyID, topicID string) (Topic, error)
	GetTopicByPattern(ctx context.Context, agencyID, pattern string) (Topic, error)
	ListTopics(ctx context.Context, agencyID string, filter TopicFilter) ([]Topic, error)
	DeleteTopic(ctx context.Context, agencyID, topicID string) error

	// Events
	RecordEvent(ctx context.Context, agencyID string, req RecordEventRequest) (Event, error)
	GetEvent(ctx context.Context, agencyID, eventID string) (Event, error)
	ListEvents(ctx context.Context, agencyID string, filter EventFilter) ([]Event, error)

	// Subscriptions
	// Subscribe is idempotent on (subscriber_service, topic_pattern): if a
	// Subscription with the same pair already exists, it is returned as-is.
	Subscribe(ctx context.Context, agencyID string, req SubscribeRequest) (Subscription, error)
	GetSubscription(ctx context.Context, agencyID, subscriptionID string) (Subscription, error)
	ListSubscriptions(ctx context.Context, agencyID string, filter SubscriptionFilter) ([]Subscription, error)
	UpdateSubscription(ctx context.Context, agencyID, subscriptionID string, req UpdateSubscriptionRequest) (Subscription, error)
	Unsubscribe(ctx context.Context, agencyID, subscriptionID string) error

	// Deliveries
	// Ack records that Cross confirmed delivery for (subscriptionID, eventID).
	// Sets Delivery.Status → "acked" and Delivery.AckedAt → now.
	// Idempotent: calling Ack on an already-acked delivery is a no-op.
	Ack(ctx context.Context, agencyID string, req AckRequest) error
	// GetSubscribersForTopic returns all active Subscriptions whose topic_pattern
	// matches the given topic string exactly. Called by Cross after every Publish.
	GetSubscribersForTopic(ctx context.Context, agencyID, topic string) ([]Subscription, error)
	// RecordDelivery creates a Delivery in "pending" state for the given
	// (subscriptionID, eventID) pair. Called internally by RecordEvent.
	RecordDelivery(ctx context.Context, agencyID, subscriptionID, eventID string) (Delivery, error)
	// MarkDelivered transitions a Delivery from "pending" to "delivered".
	// Called by Cross after a successful NotifyEvent push.
	MarkDelivered(ctx context.Context, agencyID, deliveryID string) error
}

// CrossPublisher is implemented by anything that can forward event
// notifications to CodeValdCross. Pass nil to skip forwarding.
type CrossPublisher interface {
	NotifyEvent(ctx context.Context, agencyID, topic, eventID string) error
}
