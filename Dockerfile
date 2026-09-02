# Stage 1: build with the exact patched toolchain used by CI. The OCI index
# digest prevents a mutable tag from silently changing the release inputs.
FROM golang:1.26.6-alpine3.23@sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406 AS builder

ARG APPLICATION_COMMIT=unknown

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=true -trimpath \
    -ldflags="-w -s -X github.com/BMaeda84/pv-signal-radar/internal/version.Commit=${APPLICATION_COMMIT}" \
    -o /pv-signal-radar ./cmd/server

# Stage 2: Minimal non-root runtime container. Image size is measured in CI;
# it is not asserted here because the pure-Go SQLite dependency changes it.
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

# Pin the patched OpenSSL runtime revision. If Alpine removes this revision,
# the build fails rather than silently falling back to a vulnerable package.
RUN apk add --no-cache ca-certificates tzdata libcrypto3=3.5.8-r0 libssl3=3.5.8-r0 && \
    mkdir -p /app/data/research-analyses /app/research/manifests /app/research/datasets && \
    chown -R 1000:1000 /app

WORKDIR /app

COPY --from=builder /pv-signal-radar /app/pv-signal-radar

ENV PORT=8080 \
    RESEARCH_ANALYSIS_DIR=/app/data/research-analyses

EXPOSE 8080

USER 1000:1000

# Railway and other orchestrators can use this endpoint to distinguish a live
# process from one that is actually able to serve the embedded application.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:${PORT}/api/v1/health || exit 1

ENTRYPOINT ["/app/pv-signal-radar"]
