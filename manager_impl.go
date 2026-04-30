package codevaldpubsub

import (
	"context"
	"fmt"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// manager implements [Manager] backed by an [entitygraph.DataManager].
type manager struct {
	dm  entitygraph.DataManager
	pub CrossPublisher
}

// NewManager constructs a Manager backed by dm.
// pub may be nil — Cross forwarding is then skipped.
func NewManager(dm entitygraph.DataManager, pub CrossPublisher) Manager {
	return &manager{dm: dm, pub: pub}
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
	// Verify topic pattern exists.
	topics, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"pattern": req.Topic},
	})
	if err != nil {
		return Event{}, fmt.Errorf("RecordEvent: resolve topic: %w", err)
	}
	if len(topics) == 0 {
		return Event{}, ErrTopicNotFound
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
		out = append(out, eventFromEntity(e))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

// ── Subscriptions ──────────────────────────────────────────────────────────

func (m *manager) Subscribe(ctx context.Context, agencyID string, req SubscribeRequest) (Subscription, error) {
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

	// Link to Topic if TopicID is provided.
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

func strProp(props map[string]any, key string) string {
	v, _ := props[key].(string)
	return v
}
