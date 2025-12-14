# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o clustership ./cmd/clustership

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS and ncurses for terminal
RUN apk --no-cache add ca-certificates ncurses

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/clustership .

# Copy templates
COPY templates/ ./templates/

# Create non-root user
RUN adduser -D -u 1000 clustership
USER clustership

# Set terminal environment
ENV TERM=xterm-256color

ENTRYPOINT ["./clustership"]
