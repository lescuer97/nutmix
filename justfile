# Build commands for Nutmix Admin Dashboard

# Build frontend assets and Go binary in development mode
dev: build-assets-dev build-go

# Build frontend assets and Go binary in production mode
prod: build-assets-prod build-go

# Build frontend assets in development mode (watch mode)
build-assets-dev:
    #!/usr/bin/env bash
    set -euo pipefail
    cd internal/routes/admin/static && pnpm dev 

# Build frontend assets in production mode (minified)
build-assets-prod:
    #!/usr/bin/env bash
    set -euo pipefail
    cd internal/routes/admin/static && pnpm build

# Build Go binary with templ generation
build-go:
    #!/usr/bin/env bash
    set -euo pipefail
    templ generate ./internal/routes/admin && go build -o nutmix ./cmd/nutmix

# Clean build artifacts
clean:
    #!/usr/bin/env bash
    set -euo pipefail
    cd internal/routes/admin/static && pnpm run clean
    rm -f nutmix

# Run Go application in development mode with hot reload (requires air)
run-dev: dev
    #!/usr/bin/env bash
    set -euo pipefail
    ./nutmix

# Build everything and run
run: prod run-dev

# Install frontend dependencies
deps:
    #!/usr/bin/env bash
    set -euo pipefail
    cd internal/routes/admin/static && pnpm install

# Show all available recipes
help:
    @just --list
