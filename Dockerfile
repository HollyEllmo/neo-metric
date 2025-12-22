# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (needed for some go modules) and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/neo-metric \
    ./cmd/api

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for HTTPS requests, tzdata for timezones, wget for healthcheck
RUN apk add --no-cache ca-certificates tzdata wget

# Copy binary from builder
COPY --from=builder /app/bin/neo-metric /app/neo-metric

# Copy migrations if needed (for goose)
COPY --from=builder /app/migrations /app/migrations

# Expose port
EXPOSE 8080

# Run the binary
CMD ["/app/neo-metric"]
