# Multi-stage Dockerfile for the Task Manager service
# Builder stage: compile a statically linked Go binary
FROM golang:1.25.3-alpine AS builder

# Install CA certs and git (git sometimes required for private modules)
RUN apk add --no-cache ca-certificates git

WORKDIR /src

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source and build the binary
COPY . .
# Ensure reproducible build: disable cgo, target linux amd64, strip symbol table
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /bin/taskmanager ./cmd/taskmanager

# Final stage: minimal runtime image
FROM alpine:latest

# Install CA certs for TLS and reduce image size
RUN apk add --no-cache ca-certificates

# Create non-root user for security
RUN addgroup -S app && adduser -S -G app app

# Copy the compiled binary from builder
COPY --from=builder /bin/taskmanager /bin/taskmanager

# Ensure binary is executable
RUN chmod +x /bin/taskmanager

# Expose port used by the application
EXPOSE 8080

# Run as non-root user
USER app

# Default command
ENTRYPOINT ["/bin/taskmanager"]
