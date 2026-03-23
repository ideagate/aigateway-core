.PHONY: proto-generate mock-generate db-migrate test scheduler

## proto-generate: lint proto files then generate Go stubs from .proto definitions
proto-generate:
	buf lint
	buf generate

## mock-generate: generate Go mocks from interfaces using mockery v3 config
mock-generate:
	go run github.com/vektra/mockery/v3@v3.7.0 --config .mockery.yaml

## db-migrate: run database schema migrations
db-migrate:
	go run ./cmd/db-migrate

test:
	go test -v ./...

## scheduler: run the background scheduler (job-checker cron and future jobs)
scheduler:
	go run ./cmd/scheduler
