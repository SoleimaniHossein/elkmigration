# Start with the official Golang image for building the app
FROM golang:1.20 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
RUN go build -o elkmigration .

# Production image with a lightweight base image
FROM debian:bookworm-slim

# Set the working directory in the production container
WORKDIR /

# Copy the built binary from the builder container
COPY --from=builder /app/elkmigration /elkmigration

# Expose necessary ports
EXPOSE 8080

# Set the entrypoint to run the app
ENTRYPOINT ["/elkmigration"]
