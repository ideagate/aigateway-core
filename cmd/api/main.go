package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	hellov1 "github.com/ideagate/aigateway-core/gen/hello/v1"
	aigatewaygrpc "github.com/ideagate/aigateway-core/internal/aigateway/grpcserver"
	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	"github.com/ideagate/aigateway-core/internal/aigateway/providers"
	aigatewayrepository "github.com/ideagate/aigateway-core/internal/aigateway/repository"
	aigatewayusecase "github.com/ideagate/aigateway-core/internal/aigateway/usecase"
	hellogrpc "github.com/ideagate/aigateway-core/internal/hello/grpcserver"
	hellousecase "github.com/ideagate/aigateway-core/internal/hello/usecase"
	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	platformdb "github.com/ideagate/aigateway-core/internal/platform/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

func main() {
	port := flag.Int("port", 50051, "gRPC server port")
	configPath := flag.String("config", "config/default.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := platformconfig.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ── Database ────────────────────────────────────────────────────────────
	db, err := platformdb.Open(cfg.Datastores.Postgres)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := ensureSchema(db); err != nil {
		log.Fatalf("database schema check failed: %v. run `make db-migrate` and restart the API", err)
	}

	redisClient, err := platformdb.NewRedis(cfg.Datastores.Redis)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	// ── GenAI provider ──────────────────────────────────────────────────────
	provider, err := providers.New(cfg.Providers.Gemini)
	if err != nil {
		log.Fatalf("failed to create provider: %v", err)
	}
	repo := aigatewayrepository.New(db)
	repoLock := aigatewayrepository.NewDistributedLock(redisClient)

	// ── gRPC server ─────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	aigatewayv1.RegisterAIGatewayServiceServer(srv, aigatewaygrpc.New(aigatewayusecase.New(provider, repo, repoLock)))
	hellov1.RegisterHelloServiceServer(srv, hellogrpc.New(hellousecase.New()))

	// Register reflection so tools like grpcurl can inspect the server.
	reflection.Register(srv)

	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(lis)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serveErr <- err
			return
		}

		serveErr <- nil
	}()

	log.Printf("gRPC server listening on :%d", *port)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
		return
	case <-sigCtx.Done():
		log.Printf("shutdown signal received, draining gRPC server")
	}

	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	shutdownDone := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(shutdownDone)
	}()

	const shutdownTimeout = 10 * time.Second
	select {
	case <-shutdownDone:
		log.Printf("gRPC server shut down gracefully")
	case <-time.After(shutdownTimeout):
		log.Printf("graceful shutdown timed out after %s, forcing stop", shutdownTimeout)
		srv.Stop()
		<-shutdownDone
	}

	if err := <-serveErr; err != nil {
		log.Printf("server terminated with error during shutdown: %v", err)
	}
}

func ensureSchema(db *gorm.DB) error {
	for _, model := range models.MigrationModels() {
		if !db.Migrator().HasTable(model) {
			return fmt.Errorf("required table for %T does not exist", model)
		}
	}

	return nil
}
