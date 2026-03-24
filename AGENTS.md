# AGENTS.md

## Project snapshot
- Module: `github.com/ideagate/aigateway-core` (`go.mod`), Go `1.25.8`.
- Current implementation is centered on batch merchant-description scoring with Google GenAI.
- `cmd/api/main.go` runs a gRPC server exposing `AIGatewayService`; schema migrations are run separately via `cmd/db-migrate/main.go`; background cron jobs run via `cmd/scheduler/main.go`.
- Proto definitions live in `proto/`; generated Go stubs live in `gen/` (committed to the repo).
- Services follow a layered structure: `internal/<service>/grpcserver/` (gRPC adapter) → `internal/<service>/usecase/` (business logic interface + implementation).
- Structured YAML config lives in `config/default.yaml` (committed placeholder values) and `config/local.yaml` (local overrides, gitignored); loaded via `internal/platform/config.LoadConfig()` using Viper with automatic env-var overlay.
- Runtime datastores: PostgreSQL (GORM) via `internal/platform/db.Open()` and Redis via `internal/platform/db.NewRedis()`.

## Architecture and data flow
- **AIGateway runtime flow**: `cmd/api` and `cmd/scheduler` both wire `providers.New` → `repository.New` → `repository.NewDistributedLock` → `usecase.New(provider, repo, lock)`. The provider submits/polls batch jobs via GenAI; the repository persists `BatchJob` rows in Postgres; the distributed lock (Redis) prevents duplicate processing across pods.
- **SubmitBulkChatCompletions template flow**: when a request carries a `template_id`, `resolveSubmitRequests` fetches the `PromptConfig` from the repository and calls `applyPromptConfigDefaults` to fill empty fields (model, system_instruction, temperature, json_schema_response). Metadata is merged — request keys win over config keys. Lookups are cached per-call in a local `map[string]*PromptConfig`. The method operates on `proto.Clone` copies, so the caller's slice is never mutated.
- **Scheduler flow**: `cmd/scheduler/main.go` loads YAML config, wires the same dependency stack as `cmd/api`, then starts a `robfig/cron/v3` runner. The cron schedule is read from `cfg.Scheduler.SyncBatchJobStatusCron` (`scheduler.sync_batch_job_status_cron` in YAML) and validated non-empty at startup. Currently one registered job — `sync-batch-job-status` — calls `uc.SyncBatchJobStatus(ctx)` on every cron tick and logs per-job errors without aborting the batch. Graceful shutdown waits for any in-flight tick to finish via `<-c.Stop().Done()`.

## Repo-specific coding patterns
- **Layered gRPC architecture**: every service has `internal/<service>/usecase/usecase.go` defining a `Usecase` interface; the gRPC server in `internal/<service>/grpcserver/server.go` depends only on this interface. `NotImplemented` is the standard placeholder for unbuilt usecases.
- **Mocks for usecase interfaces** are generated with mockery into sibling `_mock/` directories (example: `internal/aigateway/usecase/_mock/Usecase.go` from `internal/aigateway/usecase.Usecase`), and gRPC server tests import these generated mocks.
- **gRPC server tests** use `google.golang.org/grpc/test/bufconn` for in-process integration tests without real networking. See `internal/aigateway/grpcserver/server_test.go` for the `newTestClientConn` helper pattern: create a `bufconn.Listener`, serve on a goroutine, dial with `grpc.WithContextDialer`, clean up in `t.Cleanup`.
- **Provider interface** (`internal/aigateway/providers/providers.go`): `Provider` has `Name() string`, `SubmitBatchJob`, and `GetBatchJobStatus`. The sole implementation is `GoogleProvider` (Gemini API only, no Vertex AI). Constructed via `providers.New(cfg.Providers.Gemini)` which requires `providers.gemini.api_key` in config. Mock at `internal/aigateway/providers/_mock/Provider.go`.
- **Repository + DistributedLock interfaces** (`internal/aigateway/repository/`): `Repository` (GORM-backed) has `CreateBatchJob`, `GetActiveBatchJobs` (returns pending/submitted/processing rows), `UpdateBatchJob`, `UpsertPromptConfig`, `GetPromptConfig`, `ListPromptConfigs`, and `DeletePromptConfig`. `repository.ErrNotFound` is the sentinel for missing records — returned by `GetPromptConfig` and `DeletePromptConfig` so callers can map to `codes.NotFound` without inspecting raw DB errors. `DistributionLock` (Redis-backed via SET NX) has `Acquire(ctx, key, ttl)` and `Release(ctx, key)`. Mocks at `repository/_mock/`.
- **BatchJob model** (`internal/aigateway/models/batch_job.go`): GORM table `batch_jobs`. Status constants: `pending`, `submitted`, `processing`, `completed`, `failed`. IDs are auto-generated in `BeforeCreate` as `yymmddhhmmss<3 alphanumeric>` (15 chars, sortable). `RequestProto` stores requests as a length-delimited proto binary blob (each entry: 4-byte big-endian uint32 length followed by proto bytes — see `marshalRequests`). `ResultsJson` stores the raw JSON of `InlinedResponses` on terminal completion. Additional optional fields: `WebhookResultURL *string` and `SendWebhookResultTimestamp *time.Time` for webhook delivery tracking.
- **PromptConfig model** (`internal/aigateway/models/prompt_config.go`): GORM table `prompt_configs`. ID is caller-supplied (stable identifier). Nullable fields use `sql.NullString`/`sql.NullFloat64`: `Description`, `Model`, `SystemInstruction`, `JSONSchemaResponse`, `Temperature`. `Metadata` is a `[]byte` JSON blob encoding `map[string]string`; encode/decode via package-private `encodeMetadata`/`decodeMetadata` in the usecase package. `CreatedAt`/`UpdatedAt` are managed by GORM `autoCreateTime`/`autoUpdateTime`. Included in `models.MigrationModels()` alongside `BatchJob`.
- **Distributed locking pattern**: `syncJobStatus` acquires Redis lock keyed `aigateway:batchjob:lock:<job_id>` with `lockTTL = 2 * time.Minute` before calling the provider; skips silently if the lock is already held (another pod is processing). Always `defer`s `Release`.
- **Config loading**: `platformconfig.LoadConfig(paths ...string)` — Viper YAML with automatic env-var overlay (`DATASTORES_POSTGRES_HOST` overrides `datastores.postgres.host`, etc.). Multiple paths are merged in order; use `config/local.yaml` to override defaults locally. Both `cmd/api` and `cmd/scheduler` accept a `-config` flag (default `config/default.yaml`).
- **AIGateway usecase implementation status**: `SubmitBulkChatCompletions`, `SyncBatchJobStatus`, `UpsertPromptConfig`, `ListPromptConfigs`, and `DeletePromptConfig` are fully implemented; `GetJobStatus` and `GetJobResults` still `panic("not implemented")`.

## Developer workflows verified in this repo
- Run all tests from repo root:
  ```bash
  make test
  ```
- Run DB migrations (required before starting `cmd/api`):
  ```bash
  make db-migrate
  ```
- Regenerate interface mocks after changing usecase interfaces (configured in `.mockery.yaml`):
  ```bash
  make mock-generate
  ```
- Run the background scheduler:
  ```bash
  make scheduler
  ```
- Tests live in: `internal/aigateway/grpcserver/server_test.go` (all three batch-job RPCs, nil-usecase panic), `internal/aigateway/usecase/usecase_test.go` (`marshalRequests` round-trip, `SubmitBulkChatCompletions` happy/error paths, template fill/override/cache-reuse/not-found, `UpsertPromptConfig` metadata persistence), and `internal/aigateway/models/batch_job_test.go` (`newBatchJobID` format/uniqueness, `BeforeCreate` hook).
- Override config for local development (use `config/local.yaml` to avoid editing committed defaults):
  ```bash
  go run ./cmd/api -config config/local.yaml
  go run ./cmd/scheduler -config config/local.yaml
  ```

## gRPC and protobuf

### Directory layout
```
proto/          # hand-written .proto sources (source of truth)
  aigateway/v1/
    aigateway.proto
gen/            # generated Go code — committed, do not edit manually
  aigateway/v1/
    aigateway.pb.go
    aigateway_grpc.pb.go
buf.yaml        # buf module config (lint: STANDARD, breaking: FILE)
buf.gen.yaml    # code-gen config (remote plugins: protocolbuffers/go + grpc/go)
```

### Regenerating Go stubs
```bash
make proto-generate
```
This runs `buf lint` first, then `buf generate`. Requires `buf` ≥ v1 (`brew install bufbuild/buf/buf`).
Run this whenever any `.proto` file changes.

### Linting protos
```bash
buf lint
```

### gRPC server (`cmd/api`)
- Only `AIGatewayService` is registered in `cmd/api/main.go` via `grpc.NewServer()`.
- `cmd/api` reads YAML config via `-config` flag (default `config/default.yaml`) and requires **both** Postgres and Redis to be reachable at startup.
- `cmd/api` does not run migrations; it calls `ensureSchema` on startup and fails fast if required tables are missing.
- Each service server lives in `internal/<service>/grpcserver/server.go` and is constructed with `New(usecase)` — passing nil panics by design.
- **AIGatewayService**: `internal/aigateway/grpcserver.Server` exposes six RPCs:
  - `SubmitBulkChatCompletions` — client-streaming; collects the full stream then delegates as `[]*SubmitBulkChatCompletionsRequest` to the usecase; returns a single `job_id`.
  - `GetJobStatus` — unary; delegates `GetJobStatusRequest` → `GetJobStatusResponse` (includes `JobStatus` enum).
  - `GetJobResults` — server-streaming; usecase returns `[]*GetJobResultsResponse`, each item streamed individually (nil items skipped).
  - `UpsertPromptConfig` — unary; creates or fully replaces a prompt config by caller-supplied ID.
  - `ListPromptConfigs` — unary; returns all prompt config rows unfiltered.
  - `DeletePromptConfig` — unary; removes a prompt config by ID; returns `codes.NotFound` when missing.
- The `AIGatewayService` usecase is constructed via `aigatewayusecase.New(provider, repo, repoLock)`; `SubmitBulkChatCompletions`, `SyncBatchJobStatus`, `UpsertPromptConfig`, `ListPromptConfigs`, and `DeletePromptConfig` are implemented; `GetJobStatus` and `GetJobResults` still `panic("not implemented")`.
- gRPC health service (`grpc_health_v1`) is registered; set to `NOT_SERVING` during graceful shutdown.
- Graceful shutdown: on SIGINT/SIGTERM the server drains in-flight RPCs; after 10 seconds it force-stops.
- Starts on port `50051` by default; override with `-port`.
- `grpc/reflection` is registered so `grpcurl` works out of the box.
- Run the server:
  ```bash
  make db-migrate
  go run ./cmd/api
  # or with a custom port
  go run ./cmd/api -port=8080
  ```
- Test with grpcurl (requires `brew install grpcurl`):
  ```bash
  grpcurl -plaintext localhost:50051 list aigateway.v1.AIGatewayService
  ```

### Adding new services
1. Create `proto/<service>/v1/<service>.proto` following the `aigateway.v1` pattern.
2. Run `buf generate`.
3. Create `internal/<service>/usecase/usecase.go` defining a `Usecase` interface and a `NotImplemented` placeholder (see `internal/aigateway/usecase/usecase.go`).
4. Create `internal/<service>/grpcserver/server.go` with a `Server` struct embedding `Unimplemented<Service>Server`, a `New(usecase Usecase) *Server` constructor (panic on nil), and RPC methods delegating to `usecase`.
5. Register the server with `grpc.NewServer()` in `cmd/api/main.go`.

## What to preserve when editing
- Keep `internal/` as the boundary for reusable logic; `cmd/...` should stay thin.
- Never hand-edit files under `gen/`; always regenerate with `buf generate`.
- Keep the gRPC server layer (`grpcserver/`) depending only on the `Usecase` interface, never on concrete implementations directly.

