// git_impl_converters.go — entity→domain converters and shared graph lookup
// utilities for [pubSubManager].
package codevaldpubsub

import (
	"context"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── Entity → domain converters ────────────────────────────────────────────────

// entityToTopic maps an entitygraph.Entity of type "Topic" to [Topic].
func entityToTopic(e entitygraph.Entity) Topic {
	p := e.Properties
	return Topic{
		ID:            e.ID,
		Pattern:       entitygraph.StringProp(p, "pattern"),
		Domain:        entitygraph.StringProp(p, "domain"),
		Action:        entitygraph.StringProp(p, "action"),
		SourceService: entitygraph.StringProp(p, "source_service"),
		Description:   entitygraph.StringProp(p, "description"),
		CreatedAt:     entitygraph.StringProp(p, "created_at"),
		UpdatedAt:     entitygraph.StringProp(p, "updated_at"),
	}
}

// entityToEvent maps an entitygraph.Entity of type "Event" to [Event].
func entityToEvent(e entitygraph.Entity) Event {
	p := e.Properties
	return Event{
		ID:            e.ID,
		Topic:         entitygraph.StringProp(p, "topic"),
		Domain:        entitygraph.StringProp(p, "domain"),
		AgencyID:      entitygraph.StringProp(p, "agency_id"),
		Action:        entitygraph.StringProp(p, "action"),
		Payload:       entitygraph.StringProp(p, "payload"),
		SourceService: entitygraph.StringProp(p, "source_service"),
		PublishedAt:   entitygraph.StringProp(p, "published_at"),
		CreatedAt:     entitygraph.StringProp(p, "created_at"),
	}
}

// entityToSubscription maps an entitygraph.Entity of type "Subscription" to [Subscription].
func entityToSubscription(e entitygraph.Entity, topicID string) Subscription {
	p := e.Properties
	return Subscription{
		ID:                e.ID,
		TopicID:           topicID,
		SubscriberID:      entitygraph.StringProp(p, "subscriber_id"),
		SubscriberService: entitygraph.StringProp(p, "subscriber_service"),
		TopicPattern:      entitygraph.StringProp(p, "topic_pattern"),
		Status:            entitygraph.StringProp(p, "status"),
		CreatedAt:         entitygraph.StringProp(p, "created_at"),
		UpdatedAt:         entitygraph.StringProp(p, "updated_at"),
	}
}

// ── Shared graph helpers ──────────────────────────────────────────────────────

// resolveParentID returns the first ToID for an outbound relationship with the
// given name from entityID. Returns "" on any error or when no edge exists.
func (m *pubSubManager) resolveParentID(ctx context.Context, entityID, relName string) string {
	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     relName,
		FromID:   entityID,
	})
	if err != nil || len(rels) == 0 {
		return ""
	}
	return rels[0].ToID
}
