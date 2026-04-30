// Package codevaldpubsub manages pub/sub topics, events, and subscriptions
// for the CodeVald platform. Use [NewManager] to obtain an implementation.
package codevaldpubsub

import "context"

// Manager is the primary API surface for pub/sub operations.
type Manager interface {
	// Topics
	RegisterTopic(ctx context.Context, agencyID string, req RegisterTopicRequest) (Topic, error)
	GetTopic(ctx context.Context, agencyID, topicID string) (Topic, error)
	GetTopicByPattern(ctx context.Context, agencyID, pattern string) (Topic, error)
	ListTopics(ctx context.Context, agencyID string, filter TopicFilter) ([]Topic, error)
	DeleteTopic(ctx context.Context, agencyID, topicID string) error

	// Events
	RecordEvent(ctx context.Context, agencyID string, req RecordEventRequest) (Event, error)
	GetEvent(ctx context.Context, agencyID, eventID string) (Event, error)
	ListEvents(ctx context.Context, agencyID string, filter EventFilter) ([]Event, error)

	// Subscriptions
	Subscribe(ctx context.Context, agencyID string, req SubscribeRequest) (Subscription, error)
	GetSubscription(ctx context.Context, agencyID, subscriptionID string) (Subscription, error)
	ListSubscriptions(ctx context.Context, agencyID string, filter SubscriptionFilter) ([]Subscription, error)
	UpdateSubscription(ctx context.Context, agencyID, subscriptionID string, req UpdateSubscriptionRequest) (Subscription, error)
	Unsubscribe(ctx context.Context, agencyID, subscriptionID string) error
}

// CrossPublisher is implemented by anything that can forward event
// notifications to CodeValdCross. Pass nil to skip forwarding.
type CrossPublisher interface {
	NotifyEvent(ctx context.Context, agencyID, topic, eventID string) error
}
