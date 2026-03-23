package main

import (
	"flag"
	"fmt"
	"log"
	"net"

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

	// ── GenAI provider ──────────────────────────────────────────────────────
	provider, err := providers.New(cfg.Providers.Gemini)
	if err != nil {
		log.Fatalf("failed to create provider: %v", err)
	}
	repo := aigatewayrepository.New(db)

	// ── gRPC server ─────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	aigatewayv1.RegisterAIGatewayServiceServer(srv, aigatewaygrpc.New(aigatewayusecase.New(provider, repo)))
	hellov1.RegisterHelloServiceServer(srv, hellogrpc.New(hellousecase.New()))

	// Register reflection so tools like grpcurl can inspect the server.
	reflection.Register(srv)

	log.Printf("gRPC server listening on :%d", *port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
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
