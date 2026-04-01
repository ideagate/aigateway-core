// cmd/scheduler runs background cron jobs for the AI Gateway service.
// Currently registered jobs:
//   - sync-batch-job-status: polls active batch jobs and writes back results (config-driven cron).
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/robfig/cron/v3"

	"github.com/ideagate/aigateway-core/internal/aigateway/providers"
	aigatewayrepository "github.com/ideagate/aigateway-core/internal/aigateway/repository"
	aigatewayusecase "github.com/ideagate/aigateway-core/internal/aigateway/usecase"
	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	platformdb "github.com/ideagate/aigateway-core/internal/platform/db"
)

func main() {
	configPath := flag.String("config", "config/default.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := platformconfig.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("scheduler: failed to load config: %v", err)
	}
	dbType, sqlCfg, err := cfg.Datastores.ResolveSQLConfig()
	if err != nil {
		log.Fatalf("scheduler: failed to resolve datastore config: %v", err)
	}

	// ── Database ────────────────────────────────────────────────────────────
	db, err := platformdb.Open(dbType, sqlCfg)
	if err != nil {
		log.Fatalf("scheduler: failed to connect to database: %v", err)
	}

	// ── Redis ───────────────────────────────────────────────────────────────
	redisClient, err := platformdb.NewRedis(cfg.Datastores.Redis)
	if err != nil {
		log.Fatalf("scheduler: failed to connect to redis: %v", err)
	}

	// ── AI provider ─────────────────────────────────────────────────────────
	provider, err := providers.New(cfg.Providers.Gemini)
	if err != nil {
		log.Fatalf("scheduler: failed to create provider: %v", err)
	}

	// ── Dependencies ────────────────────────────────────────────────────────
	repo := aigatewayrepository.New(db)
	lock := aigatewayrepository.NewDistributedLock(redisClient)
	uc := aigatewayusecase.New(provider, repo, lock)

	// ── Cron ────────────────────────────────────────────────────────────────
	c := cron.New()
	syncBatchJobStatusCron := strings.TrimSpace(cfg.Scheduler.SyncBatchJobStatusCron)
	if syncBatchJobStatusCron == "" {
		log.Fatal("scheduler: scheduler.sync_batch_job_status_cron must not be empty")
	}

	// Register cron jobs from config. Add more jobs below as the scheduler grows.
	if _, err := c.AddFunc(syncBatchJobStatusCron, func() {
		ctx := context.Background()
		log.Println("scheduler: [sync-batch-job-status] tick started")
		if err := uc.SyncBatchJobStatus(ctx); err != nil {
			log.Printf("scheduler: [sync-batch-job-status] error: %v", err)
		}
		log.Println("scheduler: [sync-batch-job-status] tick finished")
	}); err != nil {
		log.Fatalf("scheduler: invalid scheduler.sync_batch_job_status_cron %q: %v", syncBatchJobStatusCron, err)
	}

	c.Start()
	log.Printf("scheduler: started with scheduler.sync_batch_job_status_cron=%q — press Ctrl+C to stop", syncBatchJobStatusCron)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("scheduler: shutting down…")
	// Stop() waits for any running cron jobs to finish before returning.
	<-c.Stop().Done()
	log.Println("scheduler: stopped")
}
