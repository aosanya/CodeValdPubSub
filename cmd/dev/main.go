// Command dev runs CodeValdPubSub locally against a local ArangoDB. Defaults
// differ from production: listens on :50052, talks to http://localhost:8529,
// and leaves CROSS_GRPC_ADDR empty so dev runs standalone (no Cross required).
// The Makefile's `make dev` target sources CodeValdPubSub/.env before exec so
// real passwords stay out of the source tree.
package main

import (
	"log"
	"os"

	"github.com/aosanya/CodeValdPubSub/internal/app"
	"github.com/aosanya/CodeValdPubSub/internal/config"
)

func main() {
	// Dev defaults — only applied when the env var is unset, so the shell or
	// .env can still override any of them.
	setDefault("PUBSUB_GRPC_LISTEN_ADDR", ":50055")
	setDefault("PUBSUB_ARANGO_ENDPOINT", "http://localhost:8529")

	log.Println("codevaldpubsub[dev]: starting with local-dev defaults")
	if err := app.Run(config.Load()); err != nil {
		log.Fatalf("codevaldpubsub[dev]: %v", err)
	}
}

func setDefault(key, val string) {
	if _, ok := os.LookupEnv(key); !ok {
		os.Setenv(key, val)
	}
}
