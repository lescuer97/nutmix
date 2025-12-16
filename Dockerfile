# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine3.22 AS builder

# Install build dependencies
RUN apk add --no-cache protobuf curl unzip bash

# Install just
RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# Set up working directory
WORKDIR /app

# Copy justfile and go module files for install-deps
COPY justfile go.mod go.sum ./

# Install all tools using just
RUN just install-deps

ENV PATH="${PATH}:/root/go/bin:/root/.bun/bin"

# Copy the rest of the source code
COPY . .

# Build using just
RUN just web-install && \
    just build

# Runtime stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bin/app .

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["./app"]
