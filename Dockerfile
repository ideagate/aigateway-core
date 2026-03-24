# syntax=docker/dockerfile:1

FROM alpine:3.20

# ca-certificates is required for TLS connections to external services (GenAI, etc.)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Build argument to select which pre-built binary to copy.
# Allowed values: api | db-migrate | scheduler
# The binary must be present at dist/<CMD_NAME> in the build context
# (produced by the CI lint-test-build step).
ARG CMD_NAME=api

COPY dist/${CMD_NAME} /app/server
# config/default.yaml ships as a baseline with placeholder values.
# Override at runtime via:
#   - Environment variables (e.g. DATASTORES_POSTGRES_HOST=<host>)
#   - A volume-mounted config file passed with -config /run/config/production.yaml
COPY config/default.yaml config/default.yaml

EXPOSE 50051

ENTRYPOINT ["/app/server"]
