# Build stage
FROM golang:1.26rc3-alpine AS builder

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o slashvibeissue .

# Final stage - using scratch
FROM scratch

# Copy the binary from builder
COPY --from=builder /build/slashvibeissue /slashvibeissue

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/slashvibeissue"]
