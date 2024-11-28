ARG GO_VERSION=1.23.3

FROM golang:${GO_VERSION}-alpine AS builder
#
## Install build dependencies
#RUN apk add --no-cache gcc musl-dev git

# Install upx
RUN apk add upx

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./

RUN go mod download

# Copy the rest of the application source code
COPY . .

# Enables module-aware mode, regardless of whether the project is inside or outside GOPATH.
ENV GO111MODULE=on
# Enable CGO and build the Go application
ENV CGO_ENABLED=1

# The -ldflags "-s -w" flags to disable the symbol table and DWARF generation that is supposed to create debugging data
RUN go build -ldflags "-s -w" -v -o elkmigration .
RUN upx -9 /app/elkmigration


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
