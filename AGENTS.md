# AGENTS.md

## Project snapshot
- Module: `github.com/ideagate/aigateway-core` (`go.mod`), Go `1.25.8`.
- Current implementation is centered on batch merchant-description scoring with Google GenAI.
- `cmd/api/main.go` runs a gRPC server exposing `HelloService` and `AIGatewayService`; schema migrations are run separately via `cmd/db-migrate/main.go`; the real scoring flow is in `cmd/merchant-scorer/main.go` calling into `internal/scorer`; background cron jobs run via `cmd/scheduler/main.go`.
- Proto definitions live in `proto/`; generated Go stubs live in `gen/` (committed to the repo).
- Services follow a layered structure: `internal/<service>/grpcserver/` (gRPC adapter) → `internal/<service>/usecase/` (business logic interface + implementation).
- Structured YAML config lives in `config/default.yaml` (committed placeholder values) and `config/local.yaml` (local overrides, gitignored); loaded via `internal/platform/config.LoadConfig()` using Viper with automatic env-var overlay.
- Runtime datastores: PostgreSQL (GORM) via `internal/platform/db.Open()` and Redis via `internal/platform/db.NewRedis()`.

## Architecture and data flow
- `cmd/merchant-scorer/main.go` is the reference entrypoint: parse flags, build `genai.ClientConfig`, create `scorer.Scorer`, optionally override model, submit sample merchants, print human-readable and JSON results.
- `internal/scorer/scorer.go` owns all scoring logic. Keep new business logic here rather than in `cmd/...`.
- `Scorer.ScoreBatch` sends **one GenAI batch job** for the whole merchant slice via `client.Batches.Create`, then polls with `waitForCompletion`, then maps batch responses back into ordered `[]ScoreResult`.
- Ordering matters: `parseResults` assumes returned `InlinedResponses` line up with input order; inputs are pre-seeded into `results` so merchant ID + original description are always echoed back.
- Prompting is intentionally centralized in the `systemPrompt` constant. Each request adds a single user prompt of the form `Score this merchant description:\n\n...` in `buildInlinedRequests`.
- **AIGateway runtime flow**: `cmd/api` and `cmd/scheduler` both wire `providers.New` → `repository.New` → `repository.NewDistributedLock` → `usecase.New(provider, repo, lock)`. The provider submits/polls batch jobs via GenAI; the repository persists `BatchJob` rows in Postgres; the distributed lock (Redis) prevents duplicate processing across pods.
- **Scheduler flow**: `cmd/scheduler/main.go` loads YAML config, wires the same dependency stack as `cmd/api`, then starts a `robfig/cron/v3` runner. Currently one registered job — `sync-batch-job-status` — calls `uc.SyncBatchJobStatus(ctx)` on every cron tick and logs per-job errors without aborting the batch. Graceful shutdown waits for any in-flight tick to finish via `<-c.Stop().Done()`.

## GenAI integration specifics
- Merchant scoring logic (`internal/scorer`) depends on `google.golang.org/genai` for runtime LLM calls.
- Two backends are supported in `buildClientConfig`:
  - `gemini`: requires `GOOGLE_API_KEY` and uses `genai.BackendGeminiAPI`.
  - `vertexai`: requires `GOOGLE_CLOUD_PROJECT`; `GOOGLE_CLOUD_LOCATION` defaults to `us-central1`; uses `genai.BackendVertexAI`.
- Model names are passed around as bare names like `gemini-2.5-flash`; the scorer prepends `models/` when building requests.
- `parseScore` is intentionally defensive against LLM formatting drift: it trims whitespace, strips fenced ```json blocks, unmarshals JSON, and rejects scores outside `[1,10]`.

## Repo-specific coding patterns
- Public data shape lives in `internal/scorer`: `MerchantInput` in, `ScoreResult` out. Reuse these types instead of redefining request/response structs elsewhere.
- Error handling is mixed-level by design: batch/job failures return a Go error from `ScoreBatch`; per-item failures are captured in `ScoreResult.Error` so partial success can still be returned.
- `WithModel` returns a shallow copy of `Scorer` sharing the same client; follow this copy-on-configure pattern if adding more per-scorer options.
- Polling behavior is fixed in constants: `pollInterval = 5s`, `maxPollAttempts = 120` (~10 minutes). If you change timeout behavior, update both constants and any docs/comments together.
- **Layered gRPC architecture**: every service has `internal/<service>/usecase/usecase.go` defining a `Usecase` interface; the gRPC server in `internal/<service>/grpcserver/server.go` depends only on this interface. `NotImplemented` is the standard placeholder for unbuilt usecases.
- **Mocks for usecase interfaces** are generated with mockery into sibling `_mock/` directories (example: `internal/aigateway/usecase/_mock/Usecase.go` from `internal/aigateway/usecase.Usecase`), and gRPC server tests import these generated mocks.
- **gRPC server tests** use `google.golang.org/grpc/test/bufconn` for in-process integration tests without real networking. See `internal/aigateway/grpcserver/server_test.go` for the `newTestClientConn` helper pattern: create a `bufconn.Listener`, serve on a goroutine, dial with `grpc.WithContextDialer`, clean up in `t.Cleanup`.
- **Provider interface** (`internal/aigateway/providers/providers.go`): `Provider` has `Name() string`, `SubmitBatchJob`, and `GetBatchJobStatus`. The sole implementation is `GoogleProvider` (Gemini API only, no Vertex AI). Constructed via `providers.New(cfg.Providers.Gemini)` which requires `providers.gemini.api_key` in config. Mock at `internal/aigateway/providers/_mock/Provider.go`.
- **Repository + DistributedLock interfaces** (`internal/aigateway/repository/`): `Repository` (GORM-backed) has `CreateBatchJob`, `GetActiveBatchJobs` (returns pending/submitted/processing rows), and `UpdateBatchJob`. `DistributionLock` (Redis-backed via SET NX) has `Acquire(ctx, key, ttl)` and `Release(ctx, key)`. Mocks at `repository/_mock/`.
- **BatchJob model** (`internal/aigateway/models/batch_job.go`): GORM table `batch_jobs`. Status constants: `pending`, `submitted`, `processing`, `completed`, `failed`. IDs are auto-generated in `BeforeCreate` as `yymmddhhmmss<3 alphanumeric>` (15 chars, sortable). `RequestProto` stores requests as a length-delimited proto binary blob (each entry: 4-byte big-endian uint32 length followed by proto bytes — see `marshalRequests`). `ResultsJson` stores the raw JSON of `InlinedResponses` on terminal completion.
- **Distributed locking pattern**: `syncJobStatus` acquires Redis lock keyed `aigateway:batchjob:lock:<job_id>` with `lockTTL = 2 * time.Minute` before calling the provider; skips silently if the lock is already held (another pod is processing). Always `defer`s `Release`.
- **Config loading**: `platformconfig.LoadConfig(paths ...string)` — Viper YAML with automatic env-var overlay (`DATASTORES_POSTGRES_HOST` overrides `datastores.postgres.host`, etc.). Multiple paths are merged in order; use `config/local.yaml` to override defaults locally. Both `cmd/api` and `cmd/scheduler` accept a `-config` flag (default `config/default.yaml`).
- **AIGateway usecase implementation status**: `SubmitBulkChatCompletions` and `SyncBatchJobStatus` are fully implemented; `GetJobStatus` and `GetJobResults` still `panic("not implemented")`.

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
- Tests live in: `internal/scorer/scorer_test.go` (prompt parsing, response extraction, request construction, empty-input), `internal/aigateway/grpcserver/server_test.go` (all three RPCs, nil-usecase panic), `internal/aigateway/usecase/usecase_test.go` (`marshalRequests` round-trip, `SubmitBulkChatCompletions` happy/error paths), and `internal/aigateway/models/batch_job_test.go` (`newBatchJobID` format/uniqueness, `BeforeCreate` hook).
- Override config for local development (use `config/local.yaml` to avoid editing committed defaults):
  ```bash
  go run ./cmd/api -config config/local.yaml
  go run ./cmd/scheduler -config config/local.yaml
  ```
- Inspect CLI flags without credentials:
  ```bash
  go run ./cmd/merchant-scorer --help
  ```
- Run the sample scorer against Gemini Developer API:
  ```bash
  export GOOGLE_API_KEY=...
  go run ./cmd/merchant-scorer
  ```
- Run the sample scorer against Vertex AI:
  ```bash
  export GOOGLE_CLOUD_PROJECT=...
  export GOOGLE_CLOUD_LOCATION=us-central1
  go run ./cmd/merchant-scorer -backend=vertexai
  ```

## gRPC and protobuf

### Directory layout
```
proto/          # hand-written .proto sources (source of truth)
  aigateway/v1/
    aigateway.proto
  hello/v1/
    hello.proto
gen/            # generated Go code — committed, do not edit manually
  aigateway/v1/
    aigateway.pb.go
    aigateway_grpc.pb.go
  hello/v1/
    hello.pb.go
    hello_grpc.pb.go
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
- Both services are registered in `cmd/api/main.go` via `grpc.NewServer()`.
- `cmd/api` reads YAML config via `-config` flag (default `config/default.yaml`) and requires **both** Postgres and Redis to be reachable at startup.
- `cmd/api` does not run migrations; it calls `ensureSchema` on startup and fails fast if required tables are missing.
- Each service server lives in `internal/<service>/grpcserver/server.go` and is constructed with `New(usecase)` — passing nil panics by design.
- **HelloService**: `internal/hello/grpcserver.Server` embeds `UnimplementedHelloServiceServer`; backed by `internal/hello/usecase.Usecase` (fully implemented — returns `"Hello, <name>!"`).
- **AIGatewayService**: `internal/aigateway/grpcserver.Server` exposes three RPCs:
  - `SubmitBulkChatCompletions` — client-streaming; collects the full stream then delegates as `[]*SubmitBulkChatCompletionsRequest` to the usecase; returns a single `job_id`.
  - `GetJobStatus` — unary; delegates `GetJobStatusRequest` → `GetJobStatusResponse` (includes `JobStatus` enum).
  - `GetJobResults` — server-streaming; usecase returns `[]*GetJobResultsResponse`, each item streamed individually (nil items skipped).
- The `AIGatewayService` usecase is constructed via `aigatewayusecase.New(provider, repo, repoLock)`; `SubmitBulkChatCompletions` and `SyncBatchJobStatus` are implemented; `GetJobStatus` and `GetJobResults` still `panic("not implemented")`.
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
  grpcurl -plaintext -d '{"name":"Alice"}' localhost:50051 hello.v1.HelloService/SayHello
  grpcurl -plaintext localhost:50051 list aigateway.v1.AIGatewayService
  ```

### Adding new services
1. Create `proto/<service>/v1/<service>.proto` following the `hello.v1` pattern.
2. Run `buf generate`.
3. Create `internal/<service>/usecase/usecase.go` defining a `Usecase` interface and a `NotImplemented` placeholder (see `internal/aigateway/usecase/usecase.go`).
4. Create `internal/<service>/grpcserver/server.go` with a `Server` struct embedding `Unimplemented<Service>Server`, a `New(usecase Usecase) *Server` constructor (panic on nil), and RPC methods delegating to `usecase`.
5. Register the server with `grpc.NewServer()` in `cmd/api/main.go`.

## What to preserve when editing
- Keep `internal/` as the boundary for reusable scoring logic; `cmd/...` should stay thin.
- Preserve the structured JSON contract in `systemPrompt` and `parseScore`; downstream code expects `score` + `reason` only.
- If adding new batch metadata or response parsing, update tests in `internal/scorer/scorer_test.go` first or alongside the change.
- Never hand-edit files under `gen/`; always regenerate with `buf generate`.
- Keep the gRPC server layer (`grpcserver/`) depending only on the `Usecase` interface, never on concrete implementations directly.

