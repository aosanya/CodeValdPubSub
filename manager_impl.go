package codevaldpubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/aosanya/CodeValdPubSub/storage/arangodb"
)

// manager implements [Manager] backed by a [arangodb.PubSubBackend].
type manager struct {
	backend arangodb.PubSubBackend
	pub     CrossPublisher
}

// NewManager constructs a Manager backed by backend.
// pub may be nil — event forwarding to CodeValdCross is then skipped.
func NewManager(backend arangodb.PubSubBackend, pub CrossPublisher) Manager {
	return &manager{backend: backend, pub: pub}
}

// ── Topics ─────────────────────────────────────────────────────────────────

func (m *manager) CreateTopic(ctx context.Context, agencyID, name, description string) (TopicRecord, error) {
	t, err := m.backend.CreateTopic(ctx, agencyID, name, description)
	if err != nil {
		if errors.Is(err, arangodb.ErrTopicAlreadyExists) {
			return TopicRecord{}, ErrTopicAlreadyExists
		}
		return TopicRecord{}, fmt.Errorf("CreateTopic: %w", err)
	}
	return backendTopicToRecord(t), nil
}

func (m *manager) GetTopic(ctx context.Context, agencyID, name string) (TopicRecord, error) {
	t, err := m.backend.GetTopic(ctx, agencyID, name)
	if err != nil {
		if errors.Is(err, arangodb.ErrTopicNotFound) {
			return TopicRecord{}, ErrTopicNotFound
		}
		return TopicRecord{}, fmt.Errorf("GetTopic: %w", err)
	}
	return backendTopicToRecord(t), nil
}

func (m *manager) ListTopics(ctx context.Context, agencyID string) ([]TopicRecord, error) {
	topics, err := m.backend.ListTopics(ctx, agencyID)
	if err != nil {
		return nil, fmt.Errorf("ListTopics: %w", err)
	}
	out := make([]TopicRecord, len(topics))
	for i, t := range topics {
		out[i] = backendTopicToRecord(t)
	}
	return out, nil
}

func (m *manager) DeleteTopic(ctx context.Context, agencyID, name string) error {
	err := m.backend.DeleteTopic(ctx, agencyID, name)
	if err != nil {
		if errors.Is(err, arangodb.ErrTopicNotFound) {
			return ErrTopicNotFound
		}
		return fmt.Errorf("DeleteTopic: %w", err)
	}
	return nil
}

// ── Messages ───────────────────────────────────────────────────────────────

func (m *manager) Publish(ctx context.Context, agencyID, topicName string, payload []byte, attributes map[string]string) (string, error) {
	// Verify the topic exists before publishing.
	if _, err := m.backend.GetTopic(ctx, agencyID, topicName); err != nil {
		if errors.Is(err, arangodb.ErrTopicNotFound) {
			return "", ErrTopicNotFound
		}
		return "", fmt.Errorf("Publish: resolve topic: %w", err)
	}
	msgID, err := m.backend.Publish(ctx, agencyID, topicName, payload, attributes)
	if err != nil {
		return "", fmt.Errorf("Publish: %w", err)
	}
	if m.pub != nil {
		_ = m.pub.NotifyPublish(ctx, agencyID, topicName, msgID)
	}
	return msgID, nil
}

func (m *manager) Pull(ctx context.Context, agencyID, subscriptionName string, maxMessages int) ([]MessageRecord, error) {
	msgs, err := m.backend.Pull(ctx, agencyID, subscriptionName, maxMessages)
	if err != nil {
		if errors.Is(err, arangodb.ErrSubscriptionNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("Pull: %w", err)
	}
	out := make([]MessageRecord, len(msgs))
	for i, msg := range msgs {
		out[i] = MessageRecord{
			ID:          msg.ID,
			AgencyID:    msg.AgencyID,
			TopicName:   msg.TopicName,
			Payload:     msg.Payload,
			Attributes:  msg.Attributes,
			PublishedAt: msg.PublishedAt,
		}
	}
	return out, nil
}

func (m *manager) Acknowledge(ctx context.Context, agencyID, subscriptionName string, messageIDs []string) error {
	err := m.backend.Acknowledge(ctx, agencyID, subscriptionName, messageIDs)
	if err != nil {
		if errors.Is(err, arangodb.ErrSubscriptionNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("Acknowledge: %w", err)
	}
	return nil
}

// ── Subscriptions ──────────────────────────────────────────────────────────

func (m *manager) Subscribe(ctx context.Context, agencyID, topicName, subscriptionName string) (SubscriptionRecord, error) {
	s, err := m.backend.CreateSubscription(ctx, agencyID, topicName, subscriptionName)
	if err != nil {
		if errors.Is(err, arangodb.ErrSubscriptionAlreadyExists) {
			return SubscriptionRecord{}, ErrSubscriptionAlreadyExists
		}
		return SubscriptionRecord{}, fmt.Errorf("Subscribe: %w", err)
	}
	return backendSubToRecord(s), nil
}

func (m *manager) GetSubscription(ctx context.Context, agencyID, subscriptionName string) (SubscriptionRecord, error) {
	s, err := m.backend.GetSubscription(ctx, agencyID, subscriptionName)
	if err != nil {
		if errors.Is(err, arangodb.ErrSubscriptionNotFound) {
			return SubscriptionRecord{}, ErrSubscriptionNotFound
		}
		return SubscriptionRecord{}, fmt.Errorf("GetSubscription: %w", err)
	}
	return backendSubToRecord(s), nil
}

func (m *manager) ListSubscriptions(ctx context.Context, agencyID, topicName string) ([]SubscriptionRecord, error) {
	subs, err := m.backend.ListSubscriptions(ctx, agencyID, topicName)
	if err != nil {
		return nil, fmt.Errorf("ListSubscriptions: %w", err)
	}
	out := make([]SubscriptionRecord, len(subs))
	for i, s := range subs {
		out[i] = backendSubToRecord(s)
	}
	return out, nil
}

func (m *manager) Unsubscribe(ctx context.Context, agencyID, subscriptionName string) error {
	err := m.backend.DeleteSubscription(ctx, agencyID, subscriptionName)
	if err != nil {
		if errors.Is(err, arangodb.ErrSubscriptionNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("Unsubscribe: %w", err)
	}
	return nil
}

// ── converters ─────────────────────────────────────────────────────────────

func backendTopicToRecord(t arangodb.Topic) TopicRecord {
	return TopicRecord{
		ID:          t.ID,
		AgencyID:    t.AgencyID,
		Name:        t.Name,
		Description: t.Description,
		CreatedAt:   t.CreatedAt,
	}
}

func backendSubToRecord(s arangodb.Subscription) SubscriptionRecord {
	return SubscriptionRecord{
		ID:        s.ID,
		AgencyID:  s.AgencyID,
		TopicName: s.TopicName,
		Name:      s.Name,
		Cursor:    s.Cursor,
		CreatedAt: s.CreatedAt,
	}
}
