package codevaldpubsub

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// manager implements [Manager] backed by an [entitygraph.DataManager].
type manager struct {
	dm  entitygraph.DataManager
	pub CrossPublisher

	mu                sync.RWMutex
	topicHashByKey    map[string]string // "agencyID:sourceService" → produces_hash
	subscribeKeysSeen map[string]bool   // "agencyID:subscriberService:topicPattern" → subscribed
}

// NewManager constructs a Manager backed by dm.
// pub may be nil — Cross forwarding is then skipped.
func NewManager(dm entitygraph.DataManager, pub CrossPublisher) Manager {
	return &manager{
		dm:                dm,
		pub:               pub,
		topicHashByKey:    make(map[string]string),
		subscribeKeysSeen: make(map[string]bool),
	}
}

// ── Topics ─────────────────────────────────────────────────────────────────

func (m *manager) RegisterTopic(ctx context.Context, agencyID string, req RegisterTopicRequest) (Topic, error) {
	existing, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"pattern": req.Pattern},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("RegisterTopic: check existing: %w", err)
	}
	if len(existing) > 0 {
		return Topic{}, ErrTopicAlreadyRegistered
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Topic",
		Properties: map[string]any{
			"pattern":        req.Pattern,
			"domain":         req.Domain,
			"action":         req.Action,
			"source_service": req.SourceService,
			"description":    req.Description,
			"created_at":     now,
			"updated_at":     now,
		},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("RegisterTopic: create: %w", err)
	}
	if m.pub != nil {
		_ = m.pub.NotifyEvent(ctx, agencyID, TopicTopicRegistered, e.ID)
	}
	return topicFromEntity(e), nil
}

func (m *manager) RegisterTopics(ctx context.Context, agencyID, sourceService, producesHash string, topics []RegisterTopicRequest) error {
	key := agencyID + ":" + sourceService
	m.mu.RLock()
	cached := m.topicHashByKey[key]
	m.mu.RUnlock()
	if cached == producesHash {
		return nil
	}
	for _, req := range topics {
		if err := m.upsertTopic(ctx, agencyID, req); err != nil {
			return fmt.Errorf("RegisterTopics: upsert %q: %w", req.Pattern, err)
		}
	}
	m.mu.Lock()
	m.topicHashByKey[key] = producesHash
	m.mu.Unlock()
	log.Printf("codevaldpubsub: RegisterTopics: upserted %d topics for %s agency=%s", len(topics), sourceService, agencyID)
	return nil
}

func (m *manager) upsertTopic(ctx context.Context, agencyID string, req RegisterTopicRequest) error {
	_, err := m.RegisterTopic(ctx, agencyID, req)
	if errors.Is(err, ErrTopicAlreadyRegistered) {
		return nil
	}
	return err
}

func (m *manager) GetTopic(ctx context.Context, agencyID, topicID string) (Topic, error) {
	e, err := m.dm.GetEntity(ctx, agencyID, topicID)
	if err != nil {
		return Topic{}, fmt.Errorf("GetTopic: %w", err)
	}
	if e.TypeID != "Topic" {
		return Topic{}, ErrTopicNotFound
	}
	return topicFromEntity(e), nil
}

func (m *manager) GetTopicByPattern(ctx context.Context, agencyID, pattern string) (Topic, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"pattern": pattern},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("GetTopicByPattern: %w", err)
	}
	if len(entities) == 0 {
		return Topic{}, ErrTopicNotFound
	}
	return topicFromEntity(entities[0]), nil
}

func (m *manager) ListTopics(ctx context.Context, agencyID string, filter TopicFilter) ([]Topic, error) {
	props := map[string]any{}
	if filter.Domain != "" {
		props["domain"] = filter.Domain
	}
	if filter.Action != "" {
		props["action"] = filter.Action
	}
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListTopics: %w", err)
	}
	out := make([]Topic, 0, len(entities))
	for _, e := range entities {
		out = append(out, topicFromEntity(e))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (m *manager) DeleteTopic(ctx context.Context, agencyID, topicID string) error {
	if err := m.dm.DeleteEntity(ctx, agencyID, topicID); err != nil {
		return fmt.Errorf("DeleteTopic: %w", err)
	}
	return nil
}

// ── Events ─────────────────────────────────────────────────────────────────

func (m *manager) RecordEvent(ctx context.Context, agencyID string, req RecordEventRequest) (Event, error) {
	log.Printf("codevaldpubsub: RecordEvent: agencyID=%q topic=%q source=%q", agencyID, req.Topic, req.SourceService)
	// Resolve the topic, lazy-registering on first publish. This makes the
	// pipeline resilient to the Cross→PubSub topic propagation gap (services
	// register producer lists with Cross; that list isn't yet forwarded to
	// PubSub on every heartbeat). Without this, every first publish for a
	// freshly-imported agency fails NotFound until the propagation lands.
	topics, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"pattern": req.Topic},
	})
	if err != nil {
		return Event{}, fmt.Errorf("RecordEvent: resolve topic: %w", err)
	}
	if len(topics) == 0 {
		log.Printf("codevaldpubsub: RecordEvent: auto-registering missing topic %q for agency %q", req.Topic, agencyID)
		if _, rerr := m.RegisterTopic(ctx, agencyID, RegisterTopicRequest{
			Pattern:       req.Topic,
			Domain:        req.Domain,
			Action:        req.Action,
			SourceService: req.SourceService,
		}); rerr != nil {
			return Event{}, fmt.Errorf("RecordEvent: auto-register topic %q: %w", req.Topic, rerr)
		}
		topics, err = m.dm.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID:   agencyID,
			TypeID:     "Topic",
			Properties: map[string]any{"pattern": req.Topic},
		})
		if err != nil {
			return Event{}, fmt.Errorf("RecordEvent: re-resolve topic after auto-register: %w", err)
		}
		if len(topics) == 0 {
			return Event{}, ErrTopicNotFound
		}
	}

	publishedAt := req.PublishedAt
	if publishedAt == "" {
		publishedAt = time.Now().UTC().Format(time.RFC3339)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	e, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Event",
		Properties: map[string]any{
			"topic":          req.Topic,
			"domain":         req.Domain,
			"agency_id":      req.AgencyID,
			"action":         req.Action,
			"payload":        req.Payload,
			"source_service": req.SourceService,
			"published_at":   publishedAt,
			"created_at":     now,
		},
	})
	if err != nil {
		return Event{}, fmt.Errorf("RecordEvent: create: %w", err)
	}

	// Link Event → Topic.
	_, _ = m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID,
		Name:     "for_topic",
		FromID:   e.ID,
		ToID:     topics[0].ID,
	})

	// Create a Delivery("pending") for every active subscriber on this topic.
	subs, err := m.GetSubscribersForTopic(ctx, agencyID, req.Topic)
	if err != nil {
		log.Printf("codevaldpubsub: RecordEvent: GetSubscribersForTopic: %v", err)
	}
	for _, sub := range subs {
		if _, derr := m.RecordDelivery(ctx, agencyID, sub.ID, e.ID); derr != nil {
			log.Printf("codevaldpubsub: RecordEvent: RecordDelivery sub=%s: %v", sub.ID, derr)
		}
	}

	if m.pub != nil {
		_ = m.pub.NotifyEvent(ctx, agencyID, req.Topic, e.ID)
	}
	return eventFromEntity(e), nil
}

func (m *manager) GetEvent(ctx context.Context, agencyID, eventID string) (Event, error) {
	e, err := m.dm.GetEntity(ctx, agencyID, eventID)
	if err != nil {
		return Event{}, ErrEventNotFound
	}
	if e.TypeID != "Event" {
		return Event{}, ErrEventNotFound
	}
	return eventFromEntity(e), nil
}

func (m *manager) ListEvents(ctx context.Context, agencyID string, filter EventFilter) ([]Event, error) {
	props := map[string]any{}
	if filter.Domain != "" {
		props["domain"] = filter.Domain
	}
	if filter.AgencyID != "" {
		props["agency_id"] = filter.AgencyID
	}
	if filter.Action != "" {
		props["action"] = filter.Action
	}
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Event",
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListEvents: %w", err)
	}
	out := make([]Event, 0, len(entities))
	for _, e := range entities {
		ev := eventFromEntity(e)
		if filter.AfterTimestamp != "" && ev.CreatedAt < filter.AfterTimestamp {
			continue
		}
		out = append(out, ev)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

// ── Subscriptions ──────────────────────────────────────────────────────────

func (m *manager) Subscribe(ctx context.Context, agencyID string, req SubscribeRequest) (Subscription, error) {
	// Idempotent on (subscriber_service, topic_pattern) using the same in-memory
	// cache strategy as RegisterTopics: after the first successful subscribe we
	// skip the DB entirely on repeat heartbeat calls.
	key := agencyID + ":" + req.SubscriberService + ":" + req.TopicPattern

	m.mu.RLock()
	seen := m.subscribeKeysSeen[key]
	m.mu.RUnlock()

	if seen {
		existing, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID: agencyID,
			TypeID:   "Subscription",
			Properties: map[string]any{
				"subscriber_service": req.SubscriberService,
				"topic_pattern":      req.TopicPattern,
			},
		})
		if err == nil && len(existing) > 0 {
			return subscriptionFromEntity(existing[0]), nil
		}
		// Entity was deleted — evict and fall through to re-create.
		m.mu.Lock()
		delete(m.subscribeKeysSeen, key)
		m.mu.Unlock()
	}

	// Slow path: first time seeing this key in this process lifetime.
	existing, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Subscription",
		Properties: map[string]any{
			"subscriber_service": req.SubscriberService,
			"topic_pattern":      req.TopicPattern,
		},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("Subscribe: check existing: %w", err)
	}
	if len(existing) > 0 {
		m.mu.Lock()
		m.subscribeKeysSeen[key] = true
		m.mu.Unlock()
		return subscriptionFromEntity(existing[0]), nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	e, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Subscription",
		Properties: map[string]any{
			"subscriber_id":      req.SubscriberID,
			"subscriber_service": req.SubscriberService,
			"topic_pattern":      req.TopicPattern,
			"status":             "active",
			"created_at":         now,
			"updated_at":         now,
		},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("Subscribe: create: %w", err)
	}

	m.mu.Lock()
	m.subscribeKeysSeen[key] = true
	m.mu.Unlock()

	if req.TopicID != "" {
		_, _ = m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			AgencyID: agencyID,
			Name:     "subscribes_to",
			FromID:   e.ID,
			ToID:     req.TopicID,
		})
	}

	if m.pub != nil {
		_ = m.pub.NotifyEvent(ctx, agencyID, TopicSubscriptionCreated, e.ID)
	}
	return subscriptionFromEntity(e), nil
}

func (m *manager) GetSubscription(ctx context.Context, agencyID, subscriptionID string) (Subscription, error) {
	e, err := m.dm.GetEntity(ctx, agencyID, subscriptionID)
	if err != nil {
		return Subscription{}, ErrSubscriptionNotFound
	}
	if e.TypeID != "Subscription" {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return subscriptionFromEntity(e), nil
}

func (m *manager) ListSubscriptions(ctx context.Context, agencyID string, filter SubscriptionFilter) ([]Subscription, error) {
	props := map[string]any{}
	if filter.SubscriberID != "" {
		props["subscriber_id"] = filter.SubscriberID
	}
	if filter.SubscriberService != "" {
		props["subscriber_service"] = filter.SubscriberService
	}
	if filter.Status != "" {
		props["status"] = filter.Status
	}
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListSubscriptions: %w", err)
	}
	out := make([]Subscription, 0, len(entities))
	for _, e := range entities {
		out = append(out, subscriptionFromEntity(e))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (m *manager) UpdateSubscription(ctx context.Context, agencyID, subscriptionID string, req UpdateSubscriptionRequest) (Subscription, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	e, err := m.dm.UpdateEntity(ctx, agencyID, subscriptionID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":     req.Status,
			"updated_at": now,
		},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("UpdateSubscription: %w", err)
	}
	if m.pub != nil {
		_ = m.pub.NotifyEvent(ctx, agencyID, TopicSubscriptionUpdated, subscriptionID)
	}
	return subscriptionFromEntity(e), nil
}

func (m *manager) Unsubscribe(ctx context.Context, agencyID, subscriptionID string) error {
	if err := m.dm.DeleteEntity(ctx, agencyID, subscriptionID); err != nil {
		return fmt.Errorf("Unsubscribe: %w", err)
	}
	if m.pub != nil {
		_ = m.pub.NotifyEvent(ctx, agencyID, TopicSubscriptionCancelled, subscriptionID)
	}
	return nil
}

// ── Deliveries ─────────────────────────────────────────────────────────────

func (m *manager) GetSubscribersForTopic(ctx context.Context, agencyID, topic string) ([]Subscription, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Subscription",
		Properties: map[string]any{
			"status":        "active",
			"topic_pattern": topic,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("GetSubscribersForTopic: %w", err)
	}
	out := make([]Subscription, 0, len(entities))
	for _, e := range entities {
		out = append(out, subscriptionFromEntity(e))
	}
	return out, nil
}

func (m *manager) RecordDelivery(ctx context.Context, agencyID, subscriptionID, eventID string) (Delivery, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	e, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Delivery",
		Properties: map[string]any{
			"subscription_id": subscriptionID,
			"event_id":        eventID,
			"status":          "pending",
			"attempt_count":   0,
			"created_at":      now,
			"updated_at":      now,
		},
	})
	if err != nil {
		return Delivery{}, fmt.Errorf("RecordDelivery: %w", err)
	}
	return deliveryFromEntity(e), nil
}

func (m *manager) MarkDelivered(ctx context.Context, agencyID, deliveryID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := m.dm.UpdateEntity(ctx, agencyID, deliveryID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":           "delivered",
			"last_attempted_at": now,
			"updated_at":       now,
		},
	})
	if err != nil {
		return fmt.Errorf("MarkDelivered: %w", err)
	}
	return nil
}

func (m *manager) Ack(ctx context.Context, agencyID string, req AckRequest) error {
	deliveries, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Delivery",
		Properties: map[string]any{
			"subscription_id": req.SubscriptionID,
			"event_id":        req.EventID,
		},
	})
	if err != nil {
		return fmt.Errorf("Ack: lookup: %w", err)
	}
	if len(deliveries) == 0 {
		return ErrDeliveryNotFound
	}
	d := deliveryFromEntity(deliveries[0])
	if d.Status == "acked" {
		return nil // idempotent
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = m.dm.UpdateEntity(ctx, agencyID, d.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":     "acked",
			"acked_at":   now,
			"updated_at": now,
		},
	})
	if err != nil {
		return fmt.Errorf("Ack: update: %w", err)
	}
	return nil
}

// ── entity converters ──────────────────────────────────────────────────────

func topicFromEntity(e entitygraph.Entity) Topic {
	return Topic{
		ID:            e.ID,
		Pattern:       strProp(e.Properties, "pattern"),
		Domain:        strProp(e.Properties, "domain"),
		Action:        strProp(e.Properties, "action"),
		SourceService: strProp(e.Properties, "source_service"),
		Description:   strProp(e.Properties, "description"),
		CreatedAt:     strProp(e.Properties, "created_at"),
		UpdatedAt:     strProp(e.Properties, "updated_at"),
	}
}

func eventFromEntity(e entitygraph.Entity) Event {
	return Event{
		ID:            e.ID,
		Topic:         strProp(e.Properties, "topic"),
		Domain:        strProp(e.Properties, "domain"),
		AgencyID:      strProp(e.Properties, "agency_id"),
		Action:        strProp(e.Properties, "action"),
		Payload:       strProp(e.Properties, "payload"),
		SourceService: strProp(e.Properties, "source_service"),
		PublishedAt:   strProp(e.Properties, "published_at"),
		CreatedAt:     strProp(e.Properties, "created_at"),
	}
}

func subscriptionFromEntity(e entitygraph.Entity) Subscription {
	return Subscription{
		ID:                e.ID,
		SubscriberID:      strProp(e.Properties, "subscriber_id"),
		SubscriberService: strProp(e.Properties, "subscriber_service"),
		TopicPattern:      strProp(e.Properties, "topic_pattern"),
		Status:            strProp(e.Properties, "status"),
		CreatedAt:         strProp(e.Properties, "created_at"),
		UpdatedAt:         strProp(e.Properties, "updated_at"),
	}
}

func deliveryFromEntity(e entitygraph.Entity) Delivery {
	return Delivery{
		ID:              e.ID,
		SubscriptionID:  strProp(e.Properties, "subscription_id"),
		EventID:         strProp(e.Properties, "event_id"),
		Status:          strProp(e.Properties, "status"),
		AttemptCount:    intProp(e.Properties, "attempt_count"),
		LastAttemptedAt: strProp(e.Properties, "last_attempted_at"),
		AckedAt:         strProp(e.Properties, "acked_at"),
		CreatedAt:       strProp(e.Properties, "created_at"),
		UpdatedAt:       strProp(e.Properties, "updated_at"),
	}
}

func strProp(props map[string]any, key string) string {
	v, _ := props[key].(string)
	return v
}

func intProp(props map[string]any, key string) int {
	switch v := props[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
