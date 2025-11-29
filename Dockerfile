# Build stage
FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY vsftp-exporter.go ./

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o vsftp-exporter vsftp-exporter.go

# Runtime stage
FROM alpine:latest

# Install necessary runtime tools (for netstat command)
RUN apk --no-cache add ca-certificates net-tools openssh-client

# Create non-root user
RUN addgroup -g 1000 exporter && \
    adduser -D -u 1000 -G exporter exporter

# Set working directory
WORKDIR /app

# Copy binary from build stage
COPY --from=builder /build/vsftp-exporter .

# Copy configuration template
COPY config.example.json ./config.example.json

# Change file ownership
RUN chown -R exporter:exporter /app

# Switch to non-root user
USER exporter

# Expose port
EXPOSE 9101

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9101/health || exit 1

# Start command
ENTRYPOINT ["./vsftp-exporter"]
CMD ["-config=/app/config.json"]
