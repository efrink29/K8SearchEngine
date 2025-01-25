# Use the official Go image to build the application
FROM golang:1.20 AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download Go modules
RUN go mod download

# Copy the entire application code
COPY . .

# Build the Go binary for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webcrawler webcrawler.go

# Use a minimal image for running the application
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/webcrawler .

# Add CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Ensure the binary has execution permissions
RUN chmod +x webcrawler

# Command to run the binary
ENTRYPOINT ["./webcrawler"]
