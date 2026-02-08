# Multi-stage build for minimal image size
FROM golang:1.24-bullseye AS builder

# Install build dependencies for librdkafka
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    librdkafka-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o usage-processor ./cmd/consumer

# Final stage
FROM debian:bullseye-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    librdkafka1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/usage-processor .

# Create non-root user
RUN groupadd -g 1000 processor && \
    useradd -u 1000 -g processor -s /bin/bash -m processor && \
    chown -R processor:processor /app

USER processor

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep usage-processor || exit 1

ENTRYPOINT ["/app/usage-processor"]
