ARG GO_VERSION=1.23.3

FROM golang:${GO_VERSION}-alpine AS builder
ENV EXPOSE=8080

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Install upx
RUN apk add upx

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./

RUN go mod download

# Copy the rest of the application source code
COPY . .

# The -ldflags "-s -w" flags to disable the symbol table and DWARF generation that is supposed to create debugging data
RUN go build -ldflags "-s -w" -v -o elkmigration ./cmd/main.go
RUN upx -9 /app/elkmigration


# Production image with a lightweight base image
FROM debian:bookworm-slim

# Set the working directory in the production container
WORKDIR /

# Copy the built binary from the builder container
COPY --from=builder /app/elkmigration /elkmigration

# Expose necessary ports
EXPOSE $EXPOSE

# Set the entrypoint to run the app
ENTRYPOINT ["/elkmigration"]
