// Package server implements the PubSubService gRPC handler.
// It wraps a [codevaldpubsub.Manager] and translates between proto messages
// and domain types. No business logic lives here.
package server

import (
	"context"
	"errors"
	"log"
	"time"

	codevaldpubsub "github.com/aosanya/CodeValdPubSub"
	pb "github.com/aosanya/CodeValdPubSub/gen/go/codevaldpubsub/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements pb.PubSubServiceServer.
type Server struct {
	pb.UnimplementedPubSubServiceServer
	mgr      codevaldpubsub.Manager
	agencyID string
}

// New constructs a Server for the given agency.
func New(mgr codevaldpubsub.Manager, agencyID string) *Server {
	return &Server{mgr: mgr, agencyID: agencyID}
}

// ── Events ────────────────────────────────────────────────────────────────────

// Publish records a new event in the agency's event log.
func (s *Server) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
	log.Printf("codevaldpubsub: server.Publish: agencyID=%q topic=%q source=%q", req.AgencyId, req.Topic, req.Source)
	agencyID := coalesce(req.AgencyId, s.agencyID)
	evt, err := s.mgr.RecordEvent(ctx, agencyID, codevaldpubsub.RecordEventRequest{
		Topic:         req.Topic,
		Domain:        domainFromTopic(req.Topic),
		AgencyID:      agencyID,
		Action:        actionFromTopic(req.Topic),
		Payload:       req.Payload,
		SourceService: req.Source,
		PublishedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.PublishResponse{Event: eventToProto(evt)}, nil
}

// GetEvent retrieves a specific event by entity ID.
func (s *Server) GetEvent(ctx context.Context, req *pb.GetEventRequest) (*pb.Event, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	evt, err := s.mgr.GetEvent(ctx, agencyID, req.EventId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return eventToProto(evt), nil
}

// QueryEvents lists events matching the given filters.
func (s *Server) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	evts, err := s.mgr.ListEvents(ctx, agencyID, codevaldpubsub.EventFilter{
		Domain:         domainFromTopic(req.Topic),
		Action:         actionFromTopic(req.Topic),
		Limit:          int(req.Limit),
		AfterTimestamp: req.AfterTimestamp,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.Event, len(evts))
	for i, e := range evts {
		out[i] = eventToProto(e)
	}
	return &pb.QueryEventsResponse{Events: out}, nil
}

// ── Topics ────────────────────────────────────────────────────────────────────

// RegisterTopic creates a new named topic channel.
func (s *Server) RegisterTopic(ctx context.Context, req *pb.RegisterTopicRequest) (*pb.RegisterTopicResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	topic, err := s.mgr.RegisterTopic(ctx, agencyID, codevaldpubsub.RegisterTopicRequest{
		Pattern:       req.Pattern,
		Domain:        req.Domain,
		Action:        req.Action,
		SourceService: req.SourceService,
		Description:   req.Description,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RegisterTopicResponse{Topic: topicToProto(topic)}, nil
}

// RegisterTopics upserts the full topic set for a producer service.
// Idempotent on produces_hash — no DB writes if hash matches cached value.
func (s *Server) RegisterTopics(ctx context.Context, req *pb.RegisterTopicsRequest) (*pb.RegisterTopicsResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	topics := make([]codevaldpubsub.RegisterTopicRequest, 0, len(req.Patterns))
	for _, pattern := range req.Patterns {
		topics = append(topics, codevaldpubsub.RegisterTopicRequest{
			Pattern:       pattern,
			Domain:        domainFromTopic(pattern),
			Action:        actionFromTopic(pattern),
			SourceService: req.SourceService,
		})
	}
	if err := s.mgr.RegisterTopics(ctx, agencyID, req.SourceService, req.ProducesHash, topics); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RegisterTopicsResponse{}, nil
}

// GetTopic retrieves a topic by ID.
func (s *Server) GetTopic(ctx context.Context, req *pb.GetTopicRequest) (*pb.Topic, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	topic, err := s.mgr.GetTopic(ctx, agencyID, req.TopicId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return topicToProto(topic), nil
}

// ListTopics lists topics with optional domain/action filters.
func (s *Server) ListTopics(ctx context.Context, req *pb.ListTopicsRequest) (*pb.ListTopicsResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	topics, err := s.mgr.ListTopics(ctx, agencyID, codevaldpubsub.TopicFilter{
		Domain: req.Domain,
		Action: req.Action,
		Limit:  int(req.Limit),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.Topic, len(topics))
	for i, t := range topics {
		out[i] = topicToProto(t)
	}
	return &pb.ListTopicsResponse{Topics: out}, nil
}

// DeleteTopic removes a topic.
func (s *Server) DeleteTopic(ctx context.Context, req *pb.DeleteTopicRequest) (*pb.DeleteTopicResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	if err := s.mgr.DeleteTopic(ctx, agencyID, req.TopicId); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteTopicResponse{}, nil
}

// ── Subscriptions ─────────────────────────────────────────────────────────────

// Subscribe registers a service to receive events matching a topic pattern.
func (s *Server) Subscribe(ctx context.Context, req *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	sub, err := s.mgr.Subscribe(ctx, agencyID, codevaldpubsub.SubscribeRequest{
		SubscriberID:      req.SubscriberId,
		SubscriberService: req.SubscriberService,
		TopicPattern:      req.TopicPattern,
		TopicID:           req.TopicId,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.SubscribeResponse{Subscription: subscriptionToProto(sub)}, nil
}

// GetSubscription retrieves a subscription by ID.
func (s *Server) GetSubscription(ctx context.Context, req *pb.GetSubscriptionRequest) (*pb.Subscription, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	sub, err := s.mgr.GetSubscription(ctx, agencyID, req.SubscriptionId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return subscriptionToProto(sub), nil
}

// ListSubscriptions lists subscriptions with optional filters.
func (s *Server) ListSubscriptions(ctx context.Context, req *pb.ListSubscriptionsRequest) (*pb.ListSubscriptionsResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	subs, err := s.mgr.ListSubscriptions(ctx, agencyID, codevaldpubsub.SubscriptionFilter{
		SubscriberID:      req.SubscriberId,
		SubscriberService: req.SubscriberService,
		Status:            req.Status,
		Limit:             int(req.Limit),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.Subscription, len(subs))
	for i, s := range subs {
		out[i] = subscriptionToProto(s)
	}
	return &pb.ListSubscriptionsResponse{Subscriptions: out}, nil
}

// UpdateSubscription patches a subscription's mutable fields.
func (s *Server) UpdateSubscription(ctx context.Context, req *pb.UpdateSubscriptionRequest) (*pb.UpdateSubscriptionResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	sub, err := s.mgr.UpdateSubscription(ctx, agencyID, req.SubscriptionId, codevaldpubsub.UpdateSubscriptionRequest{
		Status: req.Status,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateSubscriptionResponse{Subscription: subscriptionToProto(sub)}, nil
}

// Unsubscribe cancels a subscription.
func (s *Server) Unsubscribe(ctx context.Context, req *pb.UnsubscribeRequest) (*pb.UnsubscribeResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	if err := s.mgr.Unsubscribe(ctx, agencyID, req.SubscriptionId); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UnsubscribeResponse{}, nil
}

// ── Deliveries ────────────────────────────────────────────────────────────────

// Ack records that Cross confirmed delivery for (subscriptionID, eventID).
func (s *Server) Ack(ctx context.Context, req *pb.AckRequest) (*pb.AckResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	err := s.mgr.Ack(ctx, agencyID, codevaldpubsub.AckRequest{
		SubscriptionID: req.SubscriptionId,
		EventID:        req.EventId,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.AckResponse{}, nil
}

// GetSubscribersForTopic returns all active subscriptions matching a topic.
func (s *Server) GetSubscribersForTopic(ctx context.Context, req *pb.GetSubscribersForTopicRequest) (*pb.GetSubscribersForTopicResponse, error) {
	agencyID := coalesce(req.AgencyId, s.agencyID)
	subs, err := s.mgr.GetSubscribersForTopic(ctx, agencyID, req.Topic)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.Subscription, len(subs))
	for i, sub := range subs {
		out[i] = subscriptionToProto(sub)
	}
	return &pb.GetSubscribersForTopicResponse{Subscriptions: out}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func eventToProto(e codevaldpubsub.Event) *pb.Event {
	var ts *timestamppb.Timestamp
	if t, err := time.Parse(time.RFC3339, e.PublishedAt); err == nil {
		ts = timestamppb.New(t)
	}
	return &pb.Event{
		Id:        e.ID,
		AgencyId:  e.AgencyID,
		Topic:     e.Topic,
		Source:    e.SourceService,
		Payload:   e.Payload,
		CreatedAt: ts,
	}
}

func topicToProto(t codevaldpubsub.Topic) *pb.Topic {
	out := &pb.Topic{
		Id:            t.ID,
		Pattern:       t.Pattern,
		Domain:        t.Domain,
		Action:        t.Action,
		SourceService: t.SourceService,
		Description:   t.Description,
	}
	if ts, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil {
		out.CreatedAt = timestamppb.New(ts)
	}
	if ts, err := time.Parse(time.RFC3339, t.UpdatedAt); err == nil {
		out.UpdatedAt = timestamppb.New(ts)
	}
	return out
}

func subscriptionToProto(s codevaldpubsub.Subscription) *pb.Subscription {
	out := &pb.Subscription{
		Id:                s.ID,
		SubscriberId:      s.SubscriberID,
		SubscriberService: s.SubscriberService,
		TopicPattern:      s.TopicPattern,
		Status:            s.Status,
	}
	if ts, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
		out.CreatedAt = timestamppb.New(ts)
	}
	if ts, err := time.Parse(time.RFC3339, s.UpdatedAt); err == nil {
		out.UpdatedAt = timestamppb.New(ts)
	}
	return out
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, codevaldpubsub.ErrTopicNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrTopicAlreadyRegistered):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, codevaldpubsub.ErrEventNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrDeliveryNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

func domainFromTopic(topic string) string {
	for i, c := range topic {
		if c == '.' {
			return topic[:i]
		}
	}
	return topic
}

func actionFromTopic(topic string) string {
	last := 0
	for i, c := range topic {
		if c == '.' {
			last = i + 1
		}
	}
	return topic[last:]
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
