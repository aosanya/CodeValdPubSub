// Package registrar provides the CodeValdPubSub service registrar.
// It wraps the shared-library heartbeat registrar and implements
// [codevaldpubsub.CrossPublisher] so the Manager can forward event
// notifications to CodeValdCross.
package registrar

import (
	"context"
	"log"
	"time"

	codevaldpubsub "github.com/aosanya/CodeValdPubSub"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Registrar handles two responsibilities:
//  1. Sending periodic heartbeat registrations to CodeValdCross.
//  2. Implementing [codevaldpubsub.CrossPublisher] so the Manager can
//     notify CodeValdCross when events are published.
type Registrar struct {
	heartbeat sharedregistrar.Registrar
}

// Compile-time assertion.
var _ codevaldpubsub.CrossPublisher = (*Registrar)(nil)

// New constructs a Registrar that heartbeats to CodeValdCross at crossAddr.
func New(
	crossAddr, advertiseAddr, agencyID string,
	pingInterval, pingTimeout time.Duration,
) (*Registrar, error) {
	hb, err := sharedregistrar.New(
		crossAddr,
		advertiseAddr,
		agencyID,
		"codevaldpubsub",
		codevaldpubsub.AllTopics(),
		[]string{},
		pubsubRoutes(),
		pingInterval,
		pingTimeout,
	)
	if err != nil {
		return nil, err
	}
	return &Registrar{heartbeat: hb}, nil
}

// Run starts the heartbeat loop. Must be called inside a goroutine.
func (r *Registrar) Run(ctx context.Context) {
	r.heartbeat.Run(ctx)
}

// Close releases the underlying gRPC connection.
func (r *Registrar) Close() {
	r.heartbeat.Close()
}

// NotifyEvent implements [codevaldpubsub.CrossPublisher].
// Best-effort: logs the notification and returns nil — the event has already
// been persisted and must not be rolled back.
func (r *Registrar) NotifyEvent(_ context.Context, agencyID, topic, eventID string) error {
	log.Printf("registrar[codevaldpubsub]: event topic=%q agencyID=%q eventID=%q",
		topic, agencyID, eventID)
	// TODO(CROSS-007): call OrchestratorService.Publish RPC when available.
	return nil
}

// pubsubRoutes returns HTTP routes CodeValdPubSub exposes via Cross.
func pubsubRoutes() []types.RouteInfo {
	return []types.RouteInfo{
		{Method: "POST", Pattern: "/pubsub/events", GrpcMethod: "/codevaldpubsub.v1.PubSubService/Publish"},
		{Method: "GET", Pattern: "/pubsub/events/{eventId}", GrpcMethod: "/codevaldpubsub.v1.PubSubService/GetEvent"},
		{Method: "GET", Pattern: "/pubsub/events", GrpcMethod: "/codevaldpubsub.v1.PubSubService/QueryEvents"},
	}
}
