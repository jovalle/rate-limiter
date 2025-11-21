# Multi-stage build for production-ready rate limiter
FROM golang:1.21-bookworm AS builder

# Install eBPF dependencies
RUN apt-get update && apt-get install -y \
    libbpf-dev \
    libelf-dev \
    clang \
    llvm \
    make \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate eBPF code and build
RUN go generate && go build -o rate-limiter .

# Production stage
FROM ubuntu:22.04

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -m -u 1000 -s /bin/bash ratelimiter

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/rate-limiter .

# Set ownership
RUN chown -R ratelimiter:ratelimiter /app

# Environment variables with sensible defaults
ENV INTERFACE="eth0" \
    LOG_LEVEL="info" \
    PORT="8080"

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/bin/curl", "-f", "http://localhost:8080/health", "||", "exit", "1"]

# Note: Running as non-root requires CAP_NET_ADMIN and CAP_SYS_ADMIN capabilities
# These are granted via Kubernetes security context or docker run --cap-add
USER ratelimiter

ENTRYPOINT ["./rate-limiter"]
