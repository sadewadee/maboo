# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /maboo ./cmd/maboo

# Runtime stage - minimal image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 maboo

WORKDIR /app

# Copy binary
COPY --from=builder /maboo /usr/local/bin/maboo

# Copy default config
COPY maboo.yaml.example /etc/maboo/maboo.yaml

# Create directories
RUN mkdir -p /app /var/lib/maboo/certs && \
    chown -R maboo:maboo /app /var/lib/maboo

USER maboo

EXPOSE 8080 8443

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["maboo"]
CMD ["serve", "/etc/maboo/maboo.yaml"]
