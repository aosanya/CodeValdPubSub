package codevaldpubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── in-memory fake DataManager ─────────────────────────────────────────────

type fakeDataManager struct {
	mu       sync.Mutex
	entities map[string]entitygraph.Entity // id → entity
	counter  int
}

func newFakeDM() *fakeDataManager {
	return &fakeDataManager{entities: make(map[string]entitygraph.Entity)}
}

func (f *fakeDataManager) nextID() string {
	f.counter++
	return fmt.Sprintf("id-%d", f.counter)
}

func (f *fakeDataManager) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID()
	props := make(map[string]any, len(req.Properties))
	for k, v := range req.Properties {
		props[k] = v
	}
	e := entitygraph.Entity{ID: id, AgencyID: req.AgencyID, TypeID: req.TypeID, Properties: props}
	f.entities[id] = e
	return e, nil
}

func (f *fakeDataManager) GetEntity(_ context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	return e, nil
}

func (f *fakeDataManager) UpdateEntity(_ context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	for k, v := range req.Properties {
		e.Properties[k] = v
	}
	f.entities[entityID] = e
	return e, nil
}

func (f *fakeDataManager) DeleteEntity(_ context.Context, agencyID, entityID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return entitygraph.ErrEntityNotFound
	}
	delete(f.entities, entityID)
	return nil
}

func (f *fakeDataManager) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []entitygraph.Entity
	for _, e := range f.entities {
		if e.AgencyID != filter.AgencyID {
			continue
		}
		if filter.TypeID != "" && e.TypeID != filter.TypeID {
			continue
		}
		match := true
		for k, want := range filter.Properties {
			if got, ok := e.Properties[k]; !ok || got != want {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeDataManager) UpsertEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return f.CreateEntity(context.Background(), req)
}

func (f *fakeDataManager) CreateRelationship(_ context.Context, _ entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, nil
}

func (f *fakeDataManager) GetRelationship(_ context.Context, _, _ string) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, entitygraph.ErrRelationshipNotFound
}

func (f *fakeDataManager) DeleteRelationship(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeDataManager) ListRelationships(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	return nil, nil
}

func (f *fakeDataManager) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

const testAgency = "agency-test"

func newTestManager() (Manager, *fakeDataManager) {
	dm := newFakeDM()
	return NewManager(dm, nil), dm
}

// seedDelivery inserts a Delivery entity directly into the fake DM.
func seedDelivery(t *testing.T, dm *fakeDataManager, subID, evtID, status string) string {
	t.Helper()
	e, err := dm.CreateEntity(context.Background(), entitygraph.CreateEntityRequest{
		AgencyID: testAgency,
		TypeID:   "Delivery",
		Properties: map[string]any{
			"subscription_id": subID,
			"event_id":        evtID,
			"status":          status,
		},
	})
	if err != nil {
		t.Fatalf("seedDelivery: %v", err)
	}
	return e.ID
}

// ── idempotent Subscribe ───────────────────────────────────────────────────

func TestSubscribe_Idempotent(t *testing.T) {
	m, _ := newTestManager()
	ctx := context.Background()

	req := SubscribeRequest{
		SubscriberID:      "svc-1",
		SubscriberService: "codevaldai",
		TopicPattern:      "work.task.status.changed",
	}
	s1, err := m.Subscribe(ctx, testAgency, req)
	if err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}

	// Second call with same (subscriber_service, topic_pattern) must return the same record.
	s2, err := m.Subscribe(ctx, testAgency, req)
	if err != nil {
		t.Fatalf("second Subscribe: %v", err)
	}
	if s1.ID != s2.ID {
		t.Errorf("Subscribe is not idempotent: first ID=%q second ID=%q", s1.ID, s2.ID)
	}
}

func TestSubscribe_DifferentPatternCreatesNew(t *testing.T) {
	m, _ := newTestManager()
	ctx := context.Background()

	req1 := SubscribeRequest{SubscriberService: "codevaldai", TopicPattern: "work.task.status.changed"}
	req2 := SubscribeRequest{SubscriberService: "codevaldai", TopicPattern: "git.repo.created"}

	s1, _ := m.Subscribe(ctx, testAgency, req1)
	s2, _ := m.Subscribe(ctx, testAgency, req2)

	if s1.ID == s2.ID {
		t.Error("different topic patterns should create distinct subscriptions")
	}
}

// ── GetSubscribersForTopic ─────────────────────────────────────────────────

func TestGetSubscribersForTopic_Match(t *testing.T) {
	m, _ := newTestManager()
	ctx := context.Background()

	req := SubscribeRequest{
		SubscriberID:      "svc-1",
		SubscriberService: "codevaldai",
		TopicPattern:      "work.task.status.changed",
	}
	if _, err := m.Subscribe(ctx, testAgency, req); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	subs, err := m.GetSubscribersForTopic(ctx, testAgency, "work.task.status.changed")
	if err != nil {
		t.Fatalf("GetSubscribersForTopic: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("want 1 subscriber, got %d", len(subs))
	}
	if subs[0].TopicPattern != "work.task.status.changed" {
		t.Errorf("TopicPattern = %q, want %q", subs[0].TopicPattern, "work.task.status.changed")
	}
}

func TestGetSubscribersForTopic_NoMatch(t *testing.T) {
	m, _ := newTestManager()
	ctx := context.Background()

	subs, err := m.GetSubscribersForTopic(ctx, testAgency, "work.task.status.changed")
	if err != nil {
		t.Fatalf("GetSubscribersForTopic: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("want 0 subscribers, got %d", len(subs))
	}
}

// ── Ack ────────────────────────────────────────────────────────────────────

func TestAck_Success(t *testing.T) {
	m, dm := newTestManager()
	ctx := context.Background()

	subID := "sub-1"
	evtID := "evt-1"
	seedDelivery(t, dm, subID, evtID, "pending")

	err := m.Ack(ctx, testAgency, AckRequest{SubscriptionID: subID, EventID: evtID})
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// Verify status updated to "acked".
	subs, _ := dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: testAgency,
		TypeID:   "Delivery",
		Properties: map[string]any{
			"subscription_id": subID,
			"event_id":        evtID,
		},
	})
	if len(subs) == 0 {
		t.Fatal("delivery entity not found after Ack")
	}
	if got := subs[0].Properties["status"]; got != "acked" {
		t.Errorf("status = %q, want %q", got, "acked")
	}
}

func TestAck_Idempotent(t *testing.T) {
	m, dm := newTestManager()
	ctx := context.Background()

	seedDelivery(t, dm, "sub-1", "evt-1", "acked")

	// Should not return error on already-acked delivery.
	err := m.Ack(ctx, testAgency, AckRequest{SubscriptionID: "sub-1", EventID: "evt-1"})
	if err != nil {
		t.Fatalf("Ack on already-acked delivery should be a no-op, got: %v", err)
	}
}

func TestAck_NotFound(t *testing.T) {
	m, _ := newTestManager()
	ctx := context.Background()

	err := m.Ack(ctx, testAgency, AckRequest{SubscriptionID: "sub-missing", EventID: "evt-missing"})
	if !errors.Is(err, ErrDeliveryNotFound) {
		t.Errorf("Ack missing delivery: got %v, want ErrDeliveryNotFound", err)
	}
}

// ── RecordEvent delivery fan-out ───────────────────────────────────────────

// seedTopic creates a Topic entity via RegisterTopic.
func seedTopic(t *testing.T, m Manager, pattern string) {
	t.Helper()
	if _, err := m.RegisterTopic(context.Background(), testAgency, RegisterTopicRequest{
		Pattern: pattern,
	}); err != nil {
		t.Fatalf("seedTopic %q: %v", pattern, err)
	}
}

func TestRecordEvent_CreatesDeliveryPerSubscriber(t *testing.T) {
	m, dm := newTestManager()
	ctx := context.Background()

	const topic = "work.task.status.changed"
	seedTopic(t, m, topic)

	// Register two subscribers for the topic.
	_, err := m.Subscribe(ctx, testAgency, SubscribeRequest{
		SubscriberService: "codevaldai",
		TopicPattern:      topic,
	})
	if err != nil {
		t.Fatalf("Subscribe ai: %v", err)
	}
	_, err = m.Subscribe(ctx, testAgency, SubscribeRequest{
		SubscriberService: "codevaldcomm",
		TopicPattern:      topic,
	})
	if err != nil {
		t.Fatalf("Subscribe comm: %v", err)
	}

	evt, err := m.RecordEvent(ctx, testAgency, RecordEventRequest{
		Topic:         topic,
		SourceService: "codevaldwork",
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	// Expect exactly two Delivery entities, both pending, both pointing to the event.
	deliveries, err := dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   testAgency,
		TypeID:     "Delivery",
		Properties: map[string]any{"event_id": evt.ID},
	})
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("want 2 Delivery records, got %d", len(deliveries))
	}
	for _, d := range deliveries {
		if d.Properties["status"] != "pending" {
			t.Errorf("delivery %s: status = %q, want \"pending\"", d.ID, d.Properties["status"])
		}
	}
}

func TestRecordEvent_NoSubscribersNoDelivery(t *testing.T) {
	m, dm := newTestManager()
	ctx := context.Background()

	const topic = "work.task.status.changed"
	seedTopic(t, m, topic)

	evt, err := m.RecordEvent(ctx, testAgency, RecordEventRequest{
		Topic:         topic,
		SourceService: "codevaldwork",
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	deliveries, err := dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   testAgency,
		TypeID:     "Delivery",
		Properties: map[string]any{"event_id": evt.ID},
	})
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("want 0 Delivery records, got %d", len(deliveries))
	}
}
