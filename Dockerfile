# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine3.22 AS builder

# Install build dependencies (bash needed for just installer)
RUN apk add --no-cache protobuf curl unzip bash

# Install just
RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# Install bun
RUN curl -fsSL https://bun.sh/install | bash && \
    cp /root/.bun/bin/bun /usr/local/bin/bun

# Set up working directory
WORKDIR /app

# Copy source code
COPY . .

# Install Go tools and dependencies
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 && \
    go install github.com/a-h/templ/cmd/templ@v0.3.960

# Set PATH to include Go bin directory
ENV PATH="${PATH}:/root/go/bin"

# Install web dependencies and build using just
RUN just web-install && \
    just build

# Final stage - minimal runtime image
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy only the built binary from builder stage
COPY --from=builder /app/build/nutmix /app/main

# Copy web assets if they exist
COPY --from=builder /app/internal/routes/admin/static/dist /app/internal/routes/admin/static/dist

EXPOSE 8080

CMD [ "/app/main" ]
