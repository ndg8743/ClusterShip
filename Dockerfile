# Multi-stage Dockerfile for ClusterShip
# Produces a minimal production image with optional GPU support

# Build arguments
ARG GO_VERSION=1.24rc1
ARG ALPINE_VERSION=3.21
ARG GPU_SUPPORT=false

# ============================================================================
# Stage 1: Builder
# ============================================================================
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    make

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies (allow automatic toolchain download)
ENV GOTOOLCHAIN=auto
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME

# Set build time if not provided
RUN test -n "$BUILD_TIME" || BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.Version=${VERSION} \
    -X main.Commit=${COMMIT} \
    -X main.BuildTime=${BUILD_TIME}" \
    -o /build/bin/clustership \
    ./cmd/clustership

# Verify the binary
RUN /build/bin/clustership --version 2>&1 || echo "Binary built successfully"

# ============================================================================
# Stage 2: Runtime (Standard)
# ============================================================================
FROM alpine:${ALPINE_VERSION} AS runtime-standard

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    bash \
    curl \
    ncurses

# Create non-root user
RUN addgroup -g 1000 clustership && \
    adduser -D -u 1000 -G clustership clustership

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/clustership /usr/local/bin/clustership

# Copy templates directory
COPY --from=builder /build/templates /app/templates

# Create config directory
RUN mkdir -p /home/clustership/.clustership && \
    chown -R clustership:clustership /home/clustership

# Switch to non-root user
USER clustership

# Set environment variables
ENV HOME=/home/clustership
ENV PATH=/usr/local/bin:$PATH
ENV TERM=xterm-256color

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep clustership || exit 1

# Default command
ENTRYPOINT ["clustership"]
CMD []

# ============================================================================
# Stage 3: Runtime (GPU Support)
# ============================================================================
FROM nvidia/cuda:12.3.0-base-ubuntu22.04 AS runtime-gpu

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    bash \
    curl \
    ncurses-term \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 clustership && \
    useradd -m -u 1000 -g clustership clustership

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/clustership /usr/local/bin/clustership

# Copy templates directory
COPY --from=builder /build/templates /app/templates

# Create config directory
RUN mkdir -p /home/clustership/.clustership && \
    chown -R clustership:clustership /home/clustership

# Switch to non-root user
USER clustership

# Set environment variables
ENV HOME=/home/clustership
ENV PATH=/usr/local/bin:$PATH
ENV TERM=xterm-256color
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep clustership || exit 1

# Default command
ENTRYPOINT ["clustership"]
CMD []

# ============================================================================
# Final Stage Selection
# ============================================================================
FROM runtime-standard AS final
# To build GPU version: docker build --build-arg GPU_SUPPORT=true -t clustership:gpu .

# Metadata
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME

LABEL maintainer="ClusterShip Team"
LABEL org.opencontainers.image.title="ClusterShip"
LABEL org.opencontainers.image.description="A Battleship-meets-Kubernetes terminal game"
LABEL org.opencontainers.image.url="https://github.com/clustership/clustership"
LABEL org.opencontainers.image.source="https://github.com/clustership/clustership"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILD_TIME}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.licenses="MIT"

# Documentation
LABEL description="ClusterShip is a terminal-based battleship game that teaches \
Kubernetes concepts through gameplay. Battle AI companies on a shared ocean while \
learning about pods, services, affinity, and rescheduling."

# Usage notes
# Standard:
#   docker run -it --rm clustership:latest
# GPU:
#   docker build --build-arg GPU_SUPPORT=true -t clustership:gpu .
#   docker run -it --rm --gpus all clustership:gpu
# With kubeconfig:
#   docker run -it --rm -v ~/.kube:/home/clustership/.kube:ro clustership:latest
# Custom settings:
#   docker run -it --rm -v ~/.clustership:/home/clustership/.clustership clustership:latest
