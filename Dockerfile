# Build stage
FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /build

# Copy only dependency files first
COPY go.mod go.sum ./

# Download dependencies (this layer gets cached)
RUN go mod download

# Copy source code
COPY . .

# Build the application with security flags
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:latest

# Add non-root user
RUN adduser -D appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/main .
# Copy .env file from builder stage
COPY --from=builder /build/.env .

# Use non-root user
USER appuser

# Expose application port
EXPOSE 8080

# Run the binary
CMD ["./main"]