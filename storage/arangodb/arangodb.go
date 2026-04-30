// Package arangodb implements the ArangoDB backend for CodeValdPubSub.
// All implementation logic lives in
// [github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb]; this package
// is a thin service-scoped adapter that fixes the collection and graph names
// to their CodeValdPubSub-specific values.
//
// Entity collections:
//   - pubsub_entities  — mutable entities: Agency, Topic, Subscription
//
// Infrastructure collections:
//   - pubsub_relationships    — ArangoDB edge collection for all directed graph edges
//   - pubsub_schemas_draft    — one mutable draft schema document per agency
//   - pubsub_schemas_published — immutable append-only published schema snapshots
//
// Named graph: pubsub_graph
//
// Message storage is routed to pubsub_messages via TypeDefinition.StorageCollection
// in the schema seed.
//
// Use [New] to obtain a (DataManager, SchemaManager) pair from an open database.
// Use [NewBackend] to connect and construct in a single call.
// Use [NewBackendFromDB] in tests that manage their own database lifecycle.
package arangodb

import (
	"context"
	"fmt"

	driver "github.com/arangodb/go-driver"

	"github.com/aosanya/CodeValdSharedLib/arangoutil"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	sharedadb "github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Backend is a type alias for the shared ArangoDB Backend.
// Callers holding *Backend references continue to compile unchanged.
type Backend = sharedadb.Backend

// Config is the connection parameters for the CodeValdPubSub ArangoDB backend.
// It is an alias of [sharedadb.ConnConfig]; see that type for field docs.
// NewBackend requires Database to be set (e.g. "codevaldpubsub").
type Config = sharedadb.ConnConfig

// toSharedConfig expands a CodeValdPubSub Config into a full SharedLib Config,
// filling in the fixed CodeValdPubSub-specific collection and graph names.
func toSharedConfig(cfg Config) sharedadb.Config {
	return sharedadb.Config{
		Endpoint:            cfg.Endpoint,
		Username:            cfg.Username,
		Password:            cfg.Password,
		Database:            cfg.Database,
		Schema:              cfg.Schema,
		EntityCollection:    "pubsub_entities",
		RelCollection:       "pubsub_relationships",
		SchemasDraftCol:     "pubsub_schemas_draft",
		SchemasPublishedCol: "pubsub_schemas_published",
		GraphName:           "pubsub_graph",
	}
}

// New constructs a Backend from an already-open driver.Database using the
// provided schema, ensures all collections and the named graph exist, and
// returns the Backend as both a DataManager and a SchemaManager.
func New(db driver.Database, schema types.Schema) (entitygraph.DataManager, entitygraph.SchemaManager, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("arangodb: New: database must not be nil")
	}
	scfg := toSharedConfig(Config{Schema: schema})
	return sharedadb.New(db, scfg)
}

// NewBackend connects to ArangoDB using cfg, ensures all collections exist,
// and returns a ready-to-use Backend. cfg.Database is required.
func NewBackend(cfg Config) (*Backend, error) {
	if cfg.Database == "" {
		return nil, fmt.Errorf("arangodb: NewBackend: Database must be set (e.g. \"codevaldpubsub\")")
	}
	scfg := toSharedConfig(cfg)
	return sharedadb.NewBackend(scfg)
}

// NewBackendFromDB constructs a Backend from an already-open driver.Database
// using the provided schema. Intended for tests that manage their own database
// lifecycle.
func NewBackendFromDB(db driver.Database, schema types.Schema) (*Backend, error) {
	if db == nil {
		return nil, fmt.Errorf("arangodb: NewBackendFromDB: database must not be nil")
	}
	scfg := toSharedConfig(Config{Schema: schema})
	return sharedadb.NewBackendFromDB(db, scfg)
}

// NewArangoBackend constructs a [PubSubBackend] from an existing
// entitygraph.DataManager. Use this in cmd/main.go to share the same
// DataManager between the gRPC handler and any other service components.
func NewArangoBackend(dm entitygraph.DataManager) PubSubBackend {
	return &arangoBackend{dm: dm}
}

// EnsureMessageIndexes adds a persistent ArangoDB index on
// [agency_id, topic_name, published_at] to the pubsub_messages collection.
// The call is idempotent — ArangoDB returns the existing index when one with
// the same fields already exists.
//
// Call this once at startup after [NewBackend] succeeds.
// cfg.Database is required; other fields default to their zero-value equivalents.
func EnsureMessageIndexes(ctx context.Context, cfg Config) error {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8529"
	}
	if cfg.Username == "" {
		cfg.Username = "root"
	}
	db, err := arangoutil.Connect(ctx, arangoutil.Config{
		Endpoint: cfg.Endpoint,
		Username: cfg.Username,
		Password: cfg.Password,
		Database: cfg.Database,
	})
	if err != nil {
		return fmt.Errorf("EnsureMessageIndexes: connect: %w", err)
	}
	exists, err := db.CollectionExists(ctx, "pubsub_messages")
	if err != nil {
		return fmt.Errorf("EnsureMessageIndexes: check collection: %w", err)
	}
	if !exists {
		// Collection not yet created (first-run before schema seed).
		// Skip and let the index be added on the next startup.
		return nil
	}
	col, err := db.Collection(ctx, "pubsub_messages")
	if err != nil {
		return fmt.Errorf("EnsureMessageIndexes: open collection: %w", err)
	}
	_, _, err = col.EnsurePersistentIndex(ctx,
		[]string{"agency_id", "topic_name", "published_at"},
		&driver.EnsurePersistentIndexOptions{
			Name:         "idx_agency_topic_published",
			InBackground: true,
		},
	)
	if err != nil {
		return fmt.Errorf("EnsureMessageIndexes: ensure index: %w", err)
	}
	return nil
}
