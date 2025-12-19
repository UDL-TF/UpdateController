# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the controller
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o update-controller ./cmd/controller

# Runtime stage - use TF2 image as base for SteamCMD
FROM ghcr.io/udl-tf/tf2-image:latest

# Copy the controller binary from builder
COPY --from=builder /build/update-controller /usr/local/bin/update-controller

# Set permissions
RUN chmod +x /usr/local/bin/update-controller

USER ${USER}

# Default command
ENTRYPOINT ["/usr/local/bin/update-controller"]

