// Package server implements the PubSubService gRPC handler.
// It wraps a [codevaldpubsub.Manager] and translates between proto messages
// and domain types. No business logic lives here.
package server

import (
	"context"
	"errors"
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

// Publish records a new event in the agency's event log.
func (s *Server) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
	agencyID := req.AgencyId
	if agencyID == "" {
		agencyID = s.agencyID
	}
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
	evt, err := s.mgr.GetEvent(ctx, s.agencyID, req.EventId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return eventToProto(evt), nil
}

// QueryEvents lists events matching the given filters.
func (s *Server) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	agencyID := req.AgencyId
	if agencyID == "" {
		agencyID = s.agencyID
	}
	evts, err := s.mgr.ListEvents(ctx, agencyID, codevaldpubsub.EventFilter{
		Domain: domainFromTopic(req.Topic),
		Action: actionFromTopic(req.Topic),
		Limit:  int(req.Limit),
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
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

// domainFromTopic extracts the first segment of a dot-separated topic string.
func domainFromTopic(topic string) string {
	for i, c := range topic {
		if c == '.' {
			return topic[:i]
		}
	}
	return topic
}

// actionFromTopic extracts the last segment of a dot-separated topic string.
func actionFromTopic(topic string) string {
	last := 0
	for i, c := range topic {
		if c == '.' {
			last = i + 1
		}
	}
	return topic[last:]
}
