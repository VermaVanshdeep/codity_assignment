# =============================================================================
# Multi-stage Dockerfile for Go API and Worker
#
# Stages:
#   base    — shared Go toolchain + deps
#   dev     — development with air hot-reload
#   builder — production binary compiler
#   api     — minimal production image for API server
#   worker  — minimal production image for Worker service
#
# The final images are based on scratch (zero OS overhead, no shell).
# We use a distroless/static base for CA certs needed by HTTPS calls.
# =============================================================================

# ─── Base: Go toolchain + module cache ────────────────────────────────────────
FROM golang:alpine AS base
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# ─── Dev: hot-reload with air ─────────────────────────────────────────────────
FROM base AS dev
RUN go install github.com/air-verse/air@latest
COPY . .
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# ─── Builder: compile production binaries ─────────────────────────────────────
FROM base AS builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -trimpath \
    -o /out/api     ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -trimpath \
    -o /out/worker  ./cmd/worker

# ─── API production image ─────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

# ─── Worker production image ──────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/worker /worker
ENTRYPOINT ["/worker"]
