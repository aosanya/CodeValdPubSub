package server

import (
	"context"
	"testing"

	codevaldpubsub "github.com/aosanya/CodeValdPubSub"
	pb "github.com/aosanya/CodeValdPubSub/gen/go/codevaldpubsub/v1"
)

// recordingManager captures the EventFilter passed into ListEvents so tests
// can assert that the server plumbs request fields correctly.
type recordingManager struct {
	codevaldpubsub.Manager // embed for compile coverage; nil-deref on unused methods
	lastFilter             codevaldpubsub.EventFilter
}

func (r *recordingManager) ListEvents(_ context.Context, _ string, f codevaldpubsub.EventFilter) ([]codevaldpubsub.Event, error) {
	r.lastFilter = f
	return nil, nil
}

// TestQueryEvents_PlumbsAfterIntoFilter locks in that the proto field is
// named `after` (BUG-20260610-001 — `?after=` was previously named
// `after_timestamp`, which the HTTP proxy could not map from a `?after=`
// query string and so dropped every event silently).
func TestQueryEvents_PlumbsAfterIntoFilter(t *testing.T) {
	rm := &recordingManager{}
	srv := New(rm, "agency-1")

	const afterTS = "2026-06-10T11:33:00Z"
	if _, err := srv.QueryEvents(context.Background(), &pb.QueryEventsRequest{
		AgencyId: "agency-1",
		After:    afterTS,
	}); err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}

	if rm.lastFilter.AfterTimestamp != afterTS {
		t.Errorf("After not plumbed: got %q, want %q", rm.lastFilter.AfterTimestamp, afterTS)
	}
}
