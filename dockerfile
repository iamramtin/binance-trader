# Start from the official Go image
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Set GOTOOLCHAIN to auto to handle version requirements
ENV GOTOOLCHAIN=auto

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o binance-trader ./cmd/main.go

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install ca-certificates for secure connections
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/binance-trader .

# Expose any necessary ports (if your app has a web interface)
# EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["./binance-trader"]