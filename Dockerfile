# Stage 1: Build binary. Keep the toolchain aligned with go.mod/CI; a digest
# pin remains necessary if byte-for-byte reproducibility becomes a requirement.
FROM golang:1.24-alpine3.21 AS builder

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /pv-signal-radar ./cmd/server

# Stage 2: Minimal runtime container (< 20MB)
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/data && \
    chown -R 1000:1000 /app

WORKDIR /app

COPY --from=builder /pv-signal-radar /app/pv-signal-radar

ENV PORT=8080

EXPOSE 8080

USER 1000:1000

# Railway and other orchestrators can use this endpoint to distinguish a live
# process from one that is actually able to serve the embedded application.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:${PORT}/api/v1/health || exit 1

ENTRYPOINT ["/app/pv-signal-radar"]
