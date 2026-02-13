# Kodit Go Dockerfile
# Multi-stage build with tree-sitter CGo dependencies

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGo (tree-sitter)
RUN apk add --no-cache \
    build-base \
    gcc \
    g++ \
    musl-dev \
    git

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for tree-sitter
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags "fts5" \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME} -linkmode external -extldflags '-static'" \
    -o /app/kodit \
    ./cmd/kodit

# Final stage - minimal alpine image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git

# Create non-root user
RUN addgroup -g 1000 kodit && \
    adduser -u 1000 -G kodit -s /bin/sh -D kodit

# Create data directory
RUN mkdir -p /data && chown kodit:kodit /data

# Copy binary from builder
COPY --from=builder /app/kodit /usr/local/bin/kodit

# Switch to non-root user
USER kodit

# Set working directory
WORKDIR /data

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/kodit"]
CMD ["serve"]
