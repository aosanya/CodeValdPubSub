// git.go defines the [PubSubManager] interface for CodeValdPubSub.
//
// PubSubManager is the single entry point for the pub/sub event recorder.
// It manages three entity types (Topic, Event, Subscription) stored in the
// ArangoDB entity graph and publishes its own lifecycle events via the
// CrossPublisher.
//
// Topic — a registered topic pattern, e.g. "work.*.*.*.createbranch".
// Event — an immutable, append-only record of a published event.
// Subscription — a service's registered interest in a topic pattern.
//
// The concrete [pubSubManager] implementation lives in git_impl_repo.go.
// Entity converters live in git_impl_converters.go.
// Storage is injected via [entitygraph.DataManager] so the manager is
// backend-agnostic.
package codevaldpubsub

import (
	"context"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// PubSubSchemaManager is a type alias for [entitygraph.SchemaManager].
// Used in cmd/main.go to seed [DefaultPubSubSchema] on startup via SetSchema.
type PubSubSchemaManager = entitygraph.SchemaManager

// CrossPublisher is a type alias for [eventbus.Publisher] — the SharedLib
// package that unifies the publish contract across CodeVald services.
type CrossPublisher = eventbus.Publisher

// PubSubManager is the primary interface for event pub/sub recording.
// gRPC handlers hold this interface — never the concrete type.
//
// Each PubSubManager instance is scoped to a single agency. The agencyID is
// fixed at construction time via [NewPubSubManager].
//
// Implementations must be safe for concurrent use.
type PubSubManager interface {
	// ── Topic Management ──────────────────────────────────────────────────────

	// RegisterTopic creates a new Topic entity for the given pattern.
	// Returns [ErrTopicAlreadyRegistered] if a Topic with the same pattern
	// already exists.
	RegisterTopic(ctx context.Context, req RegisterTopicRequest) (Topic, error)

	// GetTopic retrieves a Topic entity by its entitygraph ID.
	// Returns [ErrTopicNotFound] if no Topic with that ID exists.
	GetTopic(ctx context.Context, topicID string) (Topic, error)

	// GetTopicByPattern retrieves a Topic entity by its exact pattern string.
	// Returns [ErrTopicNotFound] if no Topic with that pattern exists.
	GetTopicByPattern(ctx context.Context, pattern string) (Topic, error)

	// ListTopics returns Topic entities matching the given filter.
	// An empty filter returns all registered topics.
	ListTopics(ctx context.Context, filter TopicFilter) ([]Topic, error)

	// DeleteTopic removes a Topic entity and all associated Subscription edges.
	// Returns [ErrTopicNotFound] if no Topic with that ID exists.
	DeleteTopic(ctx context.Context, topicID string) error

	// ── Event Recording ───────────────────────────────────────────────────────

	// RecordEvent creates an immutable Event entity linked to its Topic.
	// If req.PublishedAt is empty, the current UTC time is used.
	// Publishes [TopicEventRecorded] after a successful write.
	RecordEvent(ctx context.Context, req RecordEventRequest) (Event, error)

	// GetEvent retrieves an Event entity by its entitygraph ID.
	// Returns [ErrEventNotFound] if no Event with that ID exists.
	GetEvent(ctx context.Context, eventID string) (Event, error)

	// ListEvents returns Event entities matching the given filter,
	// ordered newest-first (by published_at).
	ListEvents(ctx context.Context, filter EventFilter) ([]Event, error)

	// ── Subscription Management ───────────────────────────────────────────────

	// Subscribe creates a new Subscription entity with status "active".
	// If req.TopicID is non-empty the subscription is linked to that Topic via
	// a subscribes_to edge. Returns [ErrTopicNotFound] if a TopicID is given
	// but does not exist.
	// Publishes [TopicSubscriptionCreated] after a successful write.
	Subscribe(ctx context.Context, req SubscribeRequest) (Subscription, error)

	// GetSubscription retrieves a Subscription entity by its entitygraph ID.
	// Returns [ErrSubscriptionNotFound] if no Subscription with that ID exists.
	GetSubscription(ctx context.Context, subscriptionID string) (Subscription, error)

	// ListSubscriptions returns Subscription entities matching the given filter.
	// An empty filter returns all subscriptions.
	ListSubscriptions(ctx context.Context, filter SubscriptionFilter) ([]Subscription, error)

	// UpdateSubscription updates the mutable fields of a Subscription (status).
	// Valid status values: "active", "paused", "cancelled".
	// Returns [ErrSubscriptionNotFound] if no Subscription with that ID exists.
	// Publishes [TopicSubscriptionUpdated] after a successful write.
	UpdateSubscription(ctx context.Context, subscriptionID string, req UpdateSubscriptionRequest) (Subscription, error)

	// Unsubscribe sets the Subscription status to "cancelled".
	// Returns [ErrSubscriptionNotFound] if no Subscription with that ID exists.
	// Returns [ErrSubscriptionNotCancellable] if the subscription is already cancelled.
	// Publishes [TopicSubscriptionCancelled] after a successful write.
	Unsubscribe(ctx context.Context, subscriptionID string) error
}

// pubSubManager is the concrete implementation of [PubSubManager].
// It wraps [entitygraph.DataManager] to expose pub/sub-specific convenience
// methods over the entity graph.
type pubSubManager struct {
	dm        entitygraph.DataManager // graph CRUD — injected by cmd/main.go
	sm        PubSubSchemaManager     // schema versioning — injected by cmd/main.go
	publisher eventbus.Publisher      // optional; nil = skip event publishing
	agencyID  string                  // the single agency ID for this database
}

// NewPubSubManager constructs a [PubSubManager] backed by the given
// [entitygraph.DataManager] and [PubSubSchemaManager].
// agencyID is the single agency scoped to this database instance.
// pub may be nil — cross-service events are skipped when no publisher is set.
func NewPubSubManager(
	dm entitygraph.DataManager,
	sm PubSubSchemaManager,
	pub eventbus.Publisher,
	agencyID string,
) PubSubManager {
	return &pubSubManager{
		dm:        dm,
		sm:        sm,
		publisher: pub,
		agencyID:  agencyID,
	}
}

// publish emits a typed [eventbus.Event] via the optional Publisher.
// A nil publisher is silently skipped; errors are swallowed — events are
// best-effort and must not fail the originating operation.
func (m *pubSubManager) publish(ctx context.Context, topic string, payload any) {
	eventbus.SafePublish(ctx, m.publisher, eventbus.Event{
		Topic:    topic,
		AgencyID: m.agencyID,
		Payload:  payload,
	})
}
