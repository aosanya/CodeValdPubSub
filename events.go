package codevaldpubsub

// Topic strings published by CodeValdPubSub itself.
const (
	TopicEventRecorded        = "pubsub.event.recorded"
	TopicTopicRegistered      = "pubsub.topic.registered"
	TopicSubscriptionCreated  = "pubsub.subscription.created"
	TopicSubscriptionUpdated  = "pubsub.subscription.updated"
	TopicSubscriptionCancelled = "pubsub.subscription.cancelled"
)

// AllTopics is the list of topics this service publishes about its own lifecycle.
func AllTopics() []string {
	return []string{
		TopicEventRecorded,
		TopicTopicRegistered,
		TopicSubscriptionCreated,
		TopicSubscriptionUpdated,
		TopicSubscriptionCancelled,
	}
}
