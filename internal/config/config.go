// Package config loads CodeValdPubSub runtime configuration from environment variables.
package config

import (
	"time"

	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Config holds all runtime configuration for the CodeValdPubSub service.
type Config struct {
	ListenAddr     string
	ArangoEndpoint string
	ArangoUser     string
	ArangoPassword string
	ArangoDatabase string
	CrossGRPCAddr  string
	AdvertiseAddr  string
	AgencyID       string
	PingInterval   time.Duration
	PingTimeout    time.Duration
}

// Load reads runtime configuration from environment variables.
//
// Environment variables:
//
//	PUBSUB_GRPC_LISTEN_ADDR    - full listen address (overrides PUBSUB_PORT)
//	PUBSUB_PORT                - port when PUBSUB_GRPC_LISTEN_ADDR is unset (default "50055")
//	PUBSUB_ARANGO_ENDPOINT     - ArangoDB endpoint (default "http://localhost:8529")
//	PUBSUB_ARANGO_USER         - ArangoDB username (default "root")
//	PUBSUB_ARANGO_PASSWORD     - ArangoDB password (default "")
//	PUBSUB_ARANGO_DATABASE     - ArangoDB database name (default "codevaldpubsub")
//	CROSS_GRPC_ADDR            - CodeValdCross gRPC address (default ""; disables registration)
//	PUBSUB_GRPC_ADVERTISE_ADDR - address Cross dials back on (default: ListenAddr)
//	PUBSUB_AGENCY_ID           - agency scope for this instance (default ""; all agencies)
//	CROSS_PING_INTERVAL        - heartbeat cadence, Go duration string (default "30s")
//	CROSS_PING_TIMEOUT         - per-RPC Register timeout, Go duration string (default "5s")
func Load() Config {
	port := serverutil.EnvOrDefault("PUBSUB_PORT", "50055")
	listenAddr := serverutil.EnvOrDefault("PUBSUB_GRPC_LISTEN_ADDR", ":"+port)
	return Config{
		ListenAddr:     listenAddr,
		ArangoEndpoint: serverutil.EnvOrDefault("PUBSUB_ARANGO_ENDPOINT", "http://localhost:8529"),
		ArangoUser:     serverutil.EnvOrDefault("PUBSUB_ARANGO_USER", "root"),
		ArangoPassword: serverutil.EnvOrDefault("PUBSUB_ARANGO_PASSWORD", ""),
		ArangoDatabase: serverutil.EnvOrDefault("PUBSUB_ARANGO_DATABASE", "codevaldpubsub"),
		CrossGRPCAddr:  serverutil.EnvOrDefault("CROSS_GRPC_ADDR", ""),
		AdvertiseAddr:  serverutil.EnvOrDefault("PUBSUB_GRPC_ADVERTISE_ADDR", listenAddr),
		AgencyID:       serverutil.EnvOrDefault("PUBSUB_AGENCY_ID", ""),
		PingInterval:   serverutil.ParseDurationString("CROSS_PING_INTERVAL", 30*time.Second),
		PingTimeout:    serverutil.ParseDurationString("CROSS_PING_TIMEOUT", 5*time.Second),
	}
}
