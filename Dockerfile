# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /build

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /projctr ./cmd/server/

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /projctr .
COPY config.docker.toml ./
COPY config/ ./config/

# Data directory for SQLite and mounted jobs
RUN mkdir -p /data/jobs/scored

ENV CONFIG_PATH=/app/config.docker.toml
ENV DATABASE_PATH=/data/projctr.db

EXPOSE 8090

ENTRYPOINT ["/app/projctr"]
