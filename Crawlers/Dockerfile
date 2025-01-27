# Use an official Go image as the base for building
FROM golang:1.20 AS builder

# Set the working directory
WORKDIR /app

# Copy Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the Go app
RUN go build -o /app/webcrawler .

# Use a minimal base image for the final container
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/webcrawler .

# Add CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Set the PORT environment variable
ENV PORT=8080

# Expose the application's port
EXPOSE 8080

# Run the application
CMD ["./webcrawler"]
