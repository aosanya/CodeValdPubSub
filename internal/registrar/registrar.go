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

// pubsubRoutes returns all HTTP routes CodeValdPubSub exposes via Cross.
// Pattern: /pubsub/{agencyId}/... — agencyId is extracted by Cross and
// forwarded in the gRPC request body.
func pubsubRoutes() []types.RouteInfo {
	const svc = "/codevaldpubsub.v1.PubSubService"
	return []types.RouteInfo{
		// ── Events ──────────────────────────────────────────────────────────────
		{Method: "POST", Pattern: "/pubsub/{agencyId}/events", GrpcMethod: svc + "/Publish"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/events/{eventId}", GrpcMethod: svc + "/GetEvent"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/events", GrpcMethod: svc + "/QueryEvents"},
		// ── Topics ──────────────────────────────────────────────────────────────
		{Method: "POST", Pattern: "/pubsub/{agencyId}/topics", GrpcMethod: svc + "/RegisterTopic"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/topics", GrpcMethod: svc + "/ListTopics"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/topics/{topicId}", GrpcMethod: svc + "/GetTopic"},
		{Method: "DELETE", Pattern: "/pubsub/{agencyId}/topics/{topicId}", GrpcMethod: svc + "/DeleteTopic"},
		// ── Subscriptions ────────────────────────────────────────────────────────
		{Method: "POST", Pattern: "/pubsub/{agencyId}/subscriptions", GrpcMethod: svc + "/Subscribe"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/subscriptions", GrpcMethod: svc + "/ListSubscriptions"},
		{Method: "GET", Pattern: "/pubsub/{agencyId}/subscriptions/{subscriptionId}", GrpcMethod: svc + "/GetSubscription"},
		{Method: "PUT", Pattern: "/pubsub/{agencyId}/subscriptions/{subscriptionId}", GrpcMethod: svc + "/UpdateSubscription"},
		{Method: "DELETE", Pattern: "/pubsub/{agencyId}/subscriptions/{subscriptionId}", GrpcMethod: svc + "/Unsubscribe"},
	}
}
