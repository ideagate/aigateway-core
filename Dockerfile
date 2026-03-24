# syntax=docker/dockerfile:1

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install git (needed by some go modules that use VCS metadata)
RUN apk add --no-cache git

WORKDIR /app

# Cache module downloads separately from the source build
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build argument to select which cmd binary to compile.
# Allowed values: api | db-migrate | scheduler
ARG CMD_NAME=api

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/bin/${CMD_NAME} ./cmd/${CMD_NAME}

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates is required for TLS connections to external services (GenAI, etc.)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

ARG CMD_NAME=api
ENV CMD_NAME=${CMD_NAME}

COPY --from=builder /app/bin/${CMD_NAME} /app/server
# config/default.yaml ships as a baseline with placeholder values.
# Override at runtime via:
#   - Environment variables (e.g. DATASTORES_POSTGRES_HOST=<host>)
#   - A volume-mounted config file passed with -config /run/config/production.yaml
COPY config/default.yaml config/default.yaml

EXPOSE 50051

ENTRYPOINT ["/app/server"]
