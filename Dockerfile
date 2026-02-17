# Kodit Go Dockerfile
# Multi-stage build with tree-sitter CGo dependencies

# Build stage — Debian-based for glibc (required by libtokenizers.a and libonnxruntime)
FROM golang:1.25-bookworm AS builder

# Install build dependencies for CGo (tree-sitter)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Download ORT and tokenizers libraries (cached unless .ort-version or tool source changes)
COPY .ort-version ./
COPY tools/download-ort/ ./tools/download-ort/
RUN ORT_VERSION=$(cat .ort-version) go run ./tools/download-ort

# Copy pre-built embedding model (run 'make download-model' before building the image)
COPY infrastructure/provider/models/ ./infrastructure/provider/models/

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=1 CGO_LDFLAGS="-L./lib" ORT_LIB_DIR="./lib" \
    go build -tags "fts5 ORT embed_model" \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o ./build/kodit ./cmd/kodit

# Final stage — Debian slim for native glibc support
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 kodit && \
    useradd -u 1000 -g kodit -s /bin/sh -m kodit

# Create data directory
RUN mkdir -p /data && chown kodit:kodit /data

# Copy binary and ORT library from builder
COPY --from=builder /app/build/kodit /usr/local/bin/kodit
COPY --from=builder /app/lib/libonnxruntime.so /usr/lib/

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
