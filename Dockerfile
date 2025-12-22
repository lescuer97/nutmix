# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine3.22 AS builder

ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache protobuf curl unzip bash git

# Install just
RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# Set up working directory
WORKDIR /app

# Set PATH early so bun and go tools are available after installation
ENV PATH="${PATH}:/root/go/bin:/root/.bun/bin"

# Copy all source files
COPY . .

# Install all tools using just
RUN just install-deps

# install the web dependencies
RUN just web-install

# Generate protobuf code
RUN just gen-proto

# Generate templ files
RUN just gen-templ

# Build web assets
RUN just web-build-prod


RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} just build


# Runtime stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/build/nutmix ./main

# # Copy web assets
# COPY --from=builder /app/internal/routes/admin/static/dist ./internal/routes/admin/static/dist

EXPOSE 8080

CMD ["/app/main"]
