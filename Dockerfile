# syntax=docker/dockerfile:1

FROM alpine:3.20

# ca-certificates is required for TLS connections to external services (GenAI, etc.)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# All three pre-built binaries are copied in so the image can run any of them.
# Binaries must be present at dist/ in the build context
# (produced by the CI lint-test-build step).
COPY dist/api /app/api
COPY dist/db-migrate /app/db-migrate
COPY dist/scheduler /app/scheduler

# config/default.yaml ships as a baseline with placeholder values.
# Override at runtime via:
#   - Environment variables (e.g. DATASTORES_POSTGRES_HOST=<host>)
#   - A volume-mounted config file passed with -config /run/config/production.yaml
COPY config/default.yaml config/default.yaml

EXPOSE 50051

# Build argument to select which binary to run by default.
# Allowed values: api | db-migrate | scheduler
ARG CMD_NAME=api
ENV CMD_NAME=${CMD_NAME}

ENTRYPOINT ["/bin/sh", "-c", "/app/$CMD_NAME \"$@\"", "sh"]
