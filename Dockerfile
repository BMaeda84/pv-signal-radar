# Stage 1: Build binary
FROM golang:alpine AS builder

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /pv-signal-radar ./cmd/server

# Stage 2: Minimal runtime container (< 20MB)
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /pv-signal-radar /app/pv-signal-radar

ENV PORT=8080

EXPOSE 8080

USER 1000:1000

ENTRYPOINT ["/app/pv-signal-radar"]
