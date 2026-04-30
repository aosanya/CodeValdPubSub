// Package codevaldpubsub — domain value types.
package codevaldpubsub

// ── Topics ────────────────────────────────────────────────────────────────────

// Topic is a registered named channel that groups related Events.
type Topic struct {
	ID            string
	Pattern       string // routing key pattern, e.g. "git.repo.created"
	Domain        string // coarse grouping, e.g. "git"
	Action        string // fine-grained action, e.g. "repo.created"
	SourceService string // service that owns this topic
	Description   string
	CreatedAt     string
	UpdatedAt     string
}

// RegisterTopicRequest is the input for RegisterTopic.
type RegisterTopicRequest struct {
	Pattern       string
	Domain        string
	Action        string
	SourceService string
	Description   string
}

// TopicFilter scopes a ListTopics call. Zero-value fields match all.
type TopicFilter struct {
	Domain string
	Action string
	Limit  int
}

// ── Events ────────────────────────────────────────────────────────────────────

// Event is a single immutable recorded pub/sub event.
type Event struct {
	ID            string
	Topic         string // routing key, e.g. "git.repo.created"
	Domain        string
	AgencyID      string // agency the originating entity belongs to
	Action        string
	Payload       string // JSON-encoded event body
	SourceService string // producing service
	PublishedAt   string // RFC3339 timestamp from the producer
	CreatedAt     string // RFC3339 timestamp set on storage
}

// RecordEventRequest is the input for RecordEvent.
type RecordEventRequest struct {
	Topic         string
	Domain        string
	AgencyID      string
	Action        string
	Payload       string
	SourceService string
	PublishedAt   string // optional; server stamps time.Now() when empty
}

// EventFilter scopes a ListEvents call. Zero-value fields match all.
type EventFilter struct {
	Domain   string
	AgencyID string
	Action   string
	Limit    int
}

// ── Subscriptions ─────────────────────────────────────────────────────────────

// Subscription represents a service's registration to receive events
// matching a topic pattern.
type Subscription struct {
	ID                string
	SubscriberID      string
	SubscriberService string
	TopicPattern      string
	Status            string // "active" | "paused" | "cancelled"
	CreatedAt         string
	UpdatedAt         string
}

// SubscribeRequest is the input for Subscribe.
type SubscribeRequest struct {
	SubscriberID      string
	SubscriberService string
	TopicPattern      string
	TopicID           string // optional; links to a specific Topic entity
}

// SubscriptionFilter scopes a ListSubscriptions call. Zero-value fields match all.
type SubscriptionFilter struct {
	SubscriberID      string
	SubscriberService string
	Status            string
	Limit             int
}

// UpdateSubscriptionRequest patches a Subscription's mutable fields.
type UpdateSubscriptionRequest struct {
	Status string
}
