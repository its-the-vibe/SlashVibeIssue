# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o slashvibeissue .

# Final stage (distroless)
FROM gcr.io/distroless/static-debian13:nonroot

# Copy the binary from builder
COPY --from=builder /build/slashvibeissue /slashvibeissue

USER nonroot:nonroot

ENTRYPOINT ["/slashvibeissue"]
