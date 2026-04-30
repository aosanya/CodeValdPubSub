package codevaldpubsub

// Event topic constants — the closed set CodeValdPubSub publishes about its
// own lifecycle operations.
const (
	// TopicTopicRegistered fires after a Topic entity is created by RegisterTopic.
	// Payload: [TopicRegisteredPayload].
	TopicTopicRegistered = "pubsub.topic.registered"

	// TopicEventRecorded fires after an Event entity is created by RecordEvent.
	// Payload: [EventRecordedPayload].
	TopicEventRecorded = "pubsub.event.recorded"

	// TopicSubscriptionCreated fires after a Subscription entity is created by Subscribe.
	// Payload: [SubscriptionCreatedPayload].
	TopicSubscriptionCreated = "pubsub.subscription.created"

	// TopicSubscriptionUpdated fires after a Subscription entity is updated by
	// UpdateSubscription. Payload: [SubscriptionUpdatedPayload].
	TopicSubscriptionUpdated = "pubsub.subscription.updated"

	// TopicSubscriptionCancelled fires after a Subscription is cancelled by Unsubscribe.
	// Payload: [SubscriptionCancelledPayload].
	TopicSubscriptionCancelled = "pubsub.subscription.cancelled"
)

// AllTopics is the closed list of topics this service publishes.
func AllTopics() []string {
	return []string{
		TopicTopicRegistered,
		TopicEventRecorded,
		TopicSubscriptionCreated,
		TopicSubscriptionUpdated,
		TopicSubscriptionCancelled,
	}
}

// TopicRegisteredPayload is the [eventbus.Event.Payload] for [TopicTopicRegistered].
type TopicRegisteredPayload struct {
	TopicID string
	Pattern string
}

// EventRecordedPayload is the [eventbus.Event.Payload] for [TopicEventRecorded].
type EventRecordedPayload struct {
	EventID string
	Topic   string
}

// SubscriptionCreatedPayload is the [eventbus.Event.Payload] for [TopicSubscriptionCreated].
type SubscriptionCreatedPayload struct {
	SubscriptionID    string
	SubscriberID      string
	TopicPattern      string
}

// SubscriptionUpdatedPayload is the [eventbus.Event.Payload] for [TopicSubscriptionUpdated].
type SubscriptionUpdatedPayload struct {
	SubscriptionID string
	Status         string
}

// SubscriptionCancelledPayload is the [eventbus.Event.Payload] for [TopicSubscriptionCancelled].
type SubscriptionCancelledPayload struct {
	SubscriptionID string
	SubscriberID   string
}
