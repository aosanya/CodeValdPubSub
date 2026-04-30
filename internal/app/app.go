// Package app holds the shared runtime wiring for CodeValdPubSub.
package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	codevaldpubsub "github.com/aosanya/CodeValdPubSub"
	pb "github.com/aosanya/CodeValdPubSub/gen/go/codevaldpubsub/v1"
	"github.com/aosanya/CodeValdPubSub/internal/config"
	"github.com/aosanya/CodeValdPubSub/internal/registrar"
	"github.com/aosanya/CodeValdPubSub/internal/server"
	pubsubadb "github.com/aosanya/CodeValdPubSub/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	healthpb "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldhealth/v1"
	"github.com/aosanya/CodeValdSharedLib/health"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Run starts all CodeValdPubSub subsystems and blocks until SIGINT/SIGTERM.
func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Cross registrar (optional) ────────────────────────────────────────────
	var pub codevaldpubsub.CrossPublisher
	if cfg.CrossGRPCAddr != "" {
		reg, err := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.AgencyID,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldpubsub: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			pub = reg
		}
	} else {
		log.Println("codevaldpubsub: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	// ── ArangoDB backend ──────────────────────────────────────────────────────
	backend, err := pubsubadb.NewBackend(pubsubadb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
		Schema:   codevaldpubsub.DefaultPubSubSchema(),
	})
	if err != nil {
		return fmt.Errorf("ArangoDB backend: %w", err)
	}

	// ── Message index (idempotent) ────────────────────────────────────────────
	idxCtx, idxCancel := context.WithTimeout(ctx, 15*time.Second)
	if idxErr := pubsubadb.EnsureMessageIndexes(idxCtx, pubsubadb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
	}); idxErr != nil {
		log.Printf("codevaldpubsub: EnsureMessageIndexes: %v (continuing)", idxErr)
	}
	idxCancel()

	// ── Schema seed (idempotent on startup) ───────────────────────────────────
	if cfg.AgencyID != "" {
		seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := entitygraph.SeedSchema(seedCtx, backend, cfg.AgencyID, codevaldpubsub.DefaultPubSubSchema()); err != nil {
			log.Printf("codevaldpubsub: schema seed: %v", err)
		}
		seedCancel()
	} else {
		log.Println("codevaldpubsub: PUBSUB_AGENCY_ID not set — skipping schema seed")
	}

	// ── Manager ───────────────────────────────────────────────────────────────
	mgr := codevaldpubsub.NewManager(backend, pub)

	// ── TCP listener ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.ListenAddr, err)
	}

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterPubSubServiceServer(grpcServer, server.New(mgr, cfg.AgencyID))
	healthpb.RegisterHealthServiceServer(grpcServer, health.New("codevaldpubsub"))

	// ── Signal handling ───────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("codevaldpubsub: shutdown signal received")
		cancel()
	}()

	log.Printf("codevaldpubsub: listening on %s (gRPC)", cfg.ListenAddr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("codevaldpubsub: gRPC server error: %v", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Println("codevaldpubsub: shutting down")
	grpcServer.GracefulStop()
	log.Println("codevaldpubsub: stopped")
	return nil
}
