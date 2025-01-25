# Use an official Go image for building
FROM golang:1.20 as builder

# Set the working directory
WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o manager .

# Use a minimal image for running the app
FROM debian:bullseye-slim

# Set working directory
WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/manager .

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./manager"]
