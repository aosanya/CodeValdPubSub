// Package codevaldpubsub is the pub/sub event recorder for the CodeVald
// platform. It stores published events from all CodeVald services, manages
// topic registrations, and tracks service subscriptions — all in the ArangoDB
// entity graph via CodeValdSharedLib/entitygraph.
//
// # Primary Interface
//
// [PubSubManager] is the single entry point. Construct one per process via
// [NewPubSubManager] and inject it into gRPC handlers.
//
// # Schema
//
// The entity graph schema is declared in [DefaultPubSubSchema] (schema.go).
// Seed it on startup via PubSubSchemaManager.SetSchema.
//
// # Event Flow
//
//	Publisher service ──RecordEvent──► PubSubManager ──stores──► Event entity
//	                                                └──publishes──► eventbus
//	Subscriber service ──Subscribe──► PubSubManager ──stores──► Subscription entity
package codevaldpubsub
