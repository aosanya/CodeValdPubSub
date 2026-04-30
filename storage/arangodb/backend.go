// backend.go defines PubSubBackend and implements it via arangoBackend.
//
// PubSubBackend covers the full pub-sub lifecycle:
//   - Agency provisioning (EnsureAgency)
//   - Topic CRUD
//   - Message publishing (Publish)
//   - Subscription CRUD
//   - Message delivery (Pull / Acknowledge)
//
// Entities stored in entitygraph:
//   - Agency     — root tenant entity
//   - Topic      — named pub-sub channel
//   - Subscription — named consumer of a topic with a delivery cursor
//   - Message    — published event (routed to pubsub_messages via schema seed)
//
// Construction:
//
//	b := NewArangoBackend(dm)
package arangodb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/google/uuid"
)

// Sentinel errors returned by PubSubBackend operations.
var (
	ErrTopicAlreadyExists        = errors.New("topic already exists")
	ErrTopicNotFound             = errors.New("topic not found")
	ErrSubscriptionAlreadyExists = errors.New("subscription already exists")
	ErrSubscriptionNotFound      = errors.New("subscription not found")
)

// PubSubBackend defines the storage interface for CodeValdPubSub.
type PubSubBackend interface {
	// EnsureAgency creates the Agency root entity for agencyID if absent.
	EnsureAgency(ctx context.Context, agencyID string) error

	// Topic operations.
	CreateTopic(ctx context.Context, agencyID, name, description string) (Topic, error)
	GetTopic(ctx context.Context, agencyID, name string) (Topic, error)
	ListTopics(ctx context.Context, agencyID string) ([]Topic, error)
	DeleteTopic(ctx context.Context, agencyID, name string) error

	// Publish appends a message to topicName and returns its messageID.
	Publish(ctx context.Context, agencyID, topicName string, payload []byte, attributes map[string]string) (string, error)

	// Subscription operations.
	CreateSubscription(ctx context.Context, agencyID, topicName, name string) (Subscription, error)
	GetSubscription(ctx context.Context, agencyID, name string) (Subscription, error)
	ListSubscriptions(ctx context.Context, agencyID, topicName string) ([]Subscription, error)
	DeleteSubscription(ctx context.Context, agencyID, name string) error

	// Pull returns up to maxMessages undelivered messages for the subscription.
	// Messages published after the subscription's cursor are undelivered.
	Pull(ctx context.Context, agencyID, subscriptionName string, maxMessages int) ([]Message, error)

	// Acknowledge advances the subscription cursor past the latest published_at
	// among the given messageIDs.
	Acknowledge(ctx context.Context, agencyID, subscriptionName string, messageIDs []string) error
}

// Topic is a named pub-sub channel.
type Topic struct {
	ID          string
	AgencyID    string
	Name        string
	Description string
	CreatedAt   time.Time
}

// Subscription is a named consumer of a topic.
// Cursor holds the published_at timestamp of the last acknowledged message;
// Pull returns only messages published strictly after Cursor.
type Subscription struct {
	ID        string
	AgencyID  string
	TopicName string
	Name      string
	Cursor    string
	CreatedAt time.Time
}

// Message is a published event.
type Message struct {
	ID          string
	AgencyID    string
	TopicName   string
	Payload     []byte
	Attributes  map[string]string
	PublishedAt time.Time
}

// arangoBackend implements PubSubBackend over entitygraph.DataManager.
type arangoBackend struct {
	dm entitygraph.DataManager
}

// ── Agency ────────────────────────────────────────────────────────────────────

func (b *arangoBackend) EnsureAgency(ctx context.Context, agencyID string) error {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Agency",
	})
	if err != nil {
		return fmt.Errorf("EnsureAgency %s: list: %w", agencyID, err)
	}
	if len(entities) > 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = b.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Agency",
		Properties: map[string]any{
			"name":       agencyID,
			"created_at": now,
		},
	})
	if err != nil {
		return fmt.Errorf("EnsureAgency %s: create: %w", agencyID, err)
	}
	return nil
}

// ── Topics ────────────────────────────────────────────────────────────────────

func (b *arangoBackend) CreateTopic(ctx context.Context, agencyID, name, description string) (Topic, error) {
	existing, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("CreateTopic %s/%s: check existing: %w", agencyID, name, err)
	}
	if len(existing) > 0 {
		return Topic{}, fmt.Errorf("CreateTopic %s/%s: %w", agencyID, name, ErrTopicAlreadyExists)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e, err := b.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Topic",
		Properties: map[string]any{
			"name":        name,
			"description": description,
			"created_at":  now,
		},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("CreateTopic %s/%s: create: %w", agencyID, name, err)
	}
	return topicFromEntity(agencyID, e), nil
}

func (b *arangoBackend) GetTopic(ctx context.Context, agencyID, name string) (Topic, error) {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return Topic{}, fmt.Errorf("GetTopic %s/%s: list: %w", agencyID, name, err)
	}
	if len(entities) == 0 {
		return Topic{}, fmt.Errorf("GetTopic %s/%s: %w", agencyID, name, ErrTopicNotFound)
	}
	return topicFromEntity(agencyID, entities[0]), nil
}

func (b *arangoBackend) ListTopics(ctx context.Context, agencyID string) ([]Topic, error) {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Topic",
	})
	if err != nil {
		return nil, fmt.Errorf("ListTopics %s: %w", agencyID, err)
	}
	topics := make([]Topic, len(entities))
	for i, e := range entities {
		topics[i] = topicFromEntity(agencyID, e)
	}
	return topics, nil
}

// DeleteTopic soft-deletes the Topic entity and all its Subscriptions.
func (b *arangoBackend) DeleteTopic(ctx context.Context, agencyID, name string) error {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Topic",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return fmt.Errorf("DeleteTopic %s/%s: list: %w", agencyID, name, err)
	}
	if len(entities) == 0 {
		return fmt.Errorf("DeleteTopic %s/%s: %w", agencyID, name, ErrTopicNotFound)
	}
	for _, e := range entities {
		if err := b.dm.DeleteEntity(ctx, agencyID, e.ID); err != nil {
			return fmt.Errorf("DeleteTopic %s/%s: delete entity %s: %w", agencyID, name, e.ID, err)
		}
	}
	// Cascade: soft-delete all subscriptions on this topic.
	subs, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"topic_name": name},
	})
	if err == nil {
		for _, s := range subs {
			_ = b.dm.DeleteEntity(ctx, agencyID, s.ID)
		}
	}
	return nil
}

// ── Messages ──────────────────────────────────────────────────────────────────

// Publish stores a new Message entity for topicName and returns its messageID.
// Messages are routed to pubsub_messages via TypeDefinition.StorageCollection.
func (b *arangoBackend) Publish(ctx context.Context, agencyID, topicName string, payload []byte, attributes map[string]string) (string, error) {
	messageID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	attrsJSON, err := json.Marshal(attributes)
	if err != nil {
		return "", fmt.Errorf("Publish %s/%s: marshal attributes: %w", agencyID, topicName, err)
	}
	_, err = b.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Message",
		Properties: map[string]any{
			"message_id":   messageID,
			"topic_name":   topicName,
			"payload":      base64.StdEncoding.EncodeToString(payload),
			"attributes":   string(attrsJSON),
			"published_at": now,
		},
	})
	if err != nil {
		return "", fmt.Errorf("Publish %s/%s: create: %w", agencyID, topicName, err)
	}
	return messageID, nil
}

// ── Subscriptions ─────────────────────────────────────────────────────────────

func (b *arangoBackend) CreateSubscription(ctx context.Context, agencyID, topicName, name string) (Subscription, error) {
	existing, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("CreateSubscription %s/%s: check existing: %w", agencyID, name, err)
	}
	if len(existing) > 0 {
		return Subscription{}, fmt.Errorf("CreateSubscription %s/%s: %w", agencyID, name, ErrSubscriptionAlreadyExists)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e, err := b.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Subscription",
		Properties: map[string]any{
			"name":       name,
			"topic_name": topicName,
			"cursor":     "",
			"created_at": now,
		},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("CreateSubscription %s/%s: create: %w", agencyID, name, err)
	}
	return subscriptionFromEntity(agencyID, e), nil
}

func (b *arangoBackend) GetSubscription(ctx context.Context, agencyID, name string) (Subscription, error) {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return Subscription{}, fmt.Errorf("GetSubscription %s/%s: list: %w", agencyID, name, err)
	}
	if len(entities) == 0 {
		return Subscription{}, fmt.Errorf("GetSubscription %s/%s: %w", agencyID, name, ErrSubscriptionNotFound)
	}
	return subscriptionFromEntity(agencyID, entities[0]), nil
}

func (b *arangoBackend) ListSubscriptions(ctx context.Context, agencyID, topicName string) ([]Subscription, error) {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"topic_name": topicName},
	})
	if err != nil {
		return nil, fmt.Errorf("ListSubscriptions %s/%s: %w", agencyID, topicName, err)
	}
	subs := make([]Subscription, len(entities))
	for i, e := range entities {
		subs[i] = subscriptionFromEntity(agencyID, e)
	}
	return subs, nil
}

func (b *arangoBackend) DeleteSubscription(ctx context.Context, agencyID, name string) error {
	entities, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"name": name},
	})
	if err != nil {
		return fmt.Errorf("DeleteSubscription %s/%s: list: %w", agencyID, name, err)
	}
	if len(entities) == 0 {
		return fmt.Errorf("DeleteSubscription %s/%s: %w", agencyID, name, ErrSubscriptionNotFound)
	}
	for _, e := range entities {
		if err := b.dm.DeleteEntity(ctx, agencyID, e.ID); err != nil {
			return fmt.Errorf("DeleteSubscription %s/%s: delete %s: %w", agencyID, name, e.ID, err)
		}
	}
	return nil
}

// ── Delivery ──────────────────────────────────────────────────────────────────

// Pull returns up to maxMessages messages published strictly after the
// subscription's cursor (lexicographic comparison on RFC3339Nano strings).
// Results are sorted oldest-first.
func (b *arangoBackend) Pull(ctx context.Context, agencyID, subscriptionName string, maxMessages int) ([]Message, error) {
	subs, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"name": subscriptionName},
	})
	if err != nil || len(subs) == 0 {
		return nil, fmt.Errorf("Pull %s/%s: %w", agencyID, subscriptionName, ErrSubscriptionNotFound)
	}
	sub := subs[0]
	cursor, _ := sub.Properties["cursor"].(string)
	topicName, _ := sub.Properties["topic_name"].(string)

	all, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Message",
		Properties: map[string]any{"topic_name": topicName},
	})
	if err != nil {
		return nil, fmt.Errorf("Pull %s/%s: list messages: %w", agencyID, subscriptionName, err)
	}

	var pending []Message
	for _, e := range all {
		pa, _ := e.Properties["published_at"].(string)
		if cursor == "" || pa > cursor {
			pending = append(pending, messageFromEntity(agencyID, e))
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].PublishedAt.Before(pending[j].PublishedAt)
	})
	if maxMessages > 0 && len(pending) > maxMessages {
		pending = pending[:maxMessages]
	}
	return pending, nil
}

// Acknowledge advances the subscription cursor to the latest published_at
// among the given messageIDs, when that timestamp is newer than the current cursor.
func (b *arangoBackend) Acknowledge(ctx context.Context, agencyID, subscriptionName string, messageIDs []string) error {
	subs, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     "Subscription",
		Properties: map[string]any{"name": subscriptionName},
	})
	if err != nil || len(subs) == 0 {
		return fmt.Errorf("Acknowledge %s/%s: %w", agencyID, subscriptionName, ErrSubscriptionNotFound)
	}
	sub := subs[0]
	cursor, _ := sub.Properties["cursor"].(string)

	for _, msgID := range messageIDs {
		msgs, err := b.dm.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID:   agencyID,
			TypeID:     "Message",
			Properties: map[string]any{"message_id": msgID},
		})
		if err != nil || len(msgs) == 0 {
			continue
		}
		pa, _ := msgs[0].Properties["published_at"].(string)
		if pa > cursor {
			cursor = pa
		}
	}

	_, err = b.dm.UpdateEntity(ctx, agencyID, sub.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"cursor": cursor},
	})
	if err != nil {
		return fmt.Errorf("Acknowledge %s/%s: update cursor: %w", agencyID, subscriptionName, err)
	}
	return nil
}

// ── Entity converters ─────────────────────────────────────────────────────────

func topicFromEntity(agencyID string, e entitygraph.Entity) Topic {
	return Topic{
		ID:          e.ID,
		AgencyID:    agencyID,
		Name:        strProp(e.Properties, "name"),
		Description: strProp(e.Properties, "description"),
		CreatedAt:   timeProp(e.Properties, "created_at"),
	}
}

func subscriptionFromEntity(agencyID string, e entitygraph.Entity) Subscription {
	return Subscription{
		ID:        e.ID,
		AgencyID:  agencyID,
		TopicName: strProp(e.Properties, "topic_name"),
		Name:      strProp(e.Properties, "name"),
		Cursor:    strProp(e.Properties, "cursor"),
		CreatedAt: timeProp(e.Properties, "created_at"),
	}
}

func messageFromEntity(agencyID string, e entitygraph.Entity) Message {
	payload, _ := base64.StdEncoding.DecodeString(strProp(e.Properties, "payload"))
	var attributes map[string]string
	_ = json.Unmarshal([]byte(strProp(e.Properties, "attributes")), &attributes)
	return Message{
		ID:          strProp(e.Properties, "message_id"),
		AgencyID:    agencyID,
		TopicName:   strProp(e.Properties, "topic_name"),
		Payload:     payload,
		Attributes:  attributes,
		PublishedAt: timeProp(e.Properties, "published_at"),
	}
}

func strProp(props map[string]any, key string) string {
	v, _ := props[key].(string)
	return v
}

func timeProp(props map[string]any, key string) time.Time {
	s, _ := props[key].(string)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
