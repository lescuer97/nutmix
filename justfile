# Environment variables
export PATH := env_var("PATH") + ":" + `go env GOPATH` + "/bin"

# Variables
APP_NAME := "nutmix"
DOCKER_IMAGE := "nutmix"
BUILD_DIR := "build"
RUN_ARGS := ""  # Additional arguments for running the app locally
RELEASE_DIR := "release"

PLATFORMS := "linux/amd64 linux/arm64 darwin/arm64"
# Read current version from VERSION file
MODULE := "github.com/lescuer97/nutmix"
VERSION := `cat VERSION 2>/dev/null || echo "0.0.0"`
BUILD_TIME := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
COMMIT_HASH := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`


# Default recipe
default:
    @just help

# Help recipe
help:
    just --list

# Run recipe
run: build
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running {{APP_NAME}} v{{VERSION}} locally..."
    ./{{BUILD_DIR}}/{{APP_NAME}} {{RUN_ARGS}}

# Run recipe with local dev enviroment
run-dev: build-dev
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running {{APP_NAME}} v{{VERSION}} locally..."
    ./{{BUILD_DIR}}/{{APP_NAME}} {{RUN_ARGS}}

# Build recipe
build: gen-proto gen-templ web-build-prod
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building {{APP_NAME}} v{{VERSION}}..."
    mkdir -p {{BUILD_DIR}}
    go build -ldflags="-s -w \
        -X '{{MODULE}}/internal/utils.AppVersion={{VERSION}}' \
        -X '{{MODULE}}/internal/utils.BuildTime={{BUILD_TIME}}' \
        -X '{{MODULE}}/internal/utils.GitCommit={{COMMIT_HASH}}'" \
        -trimpath -o {{BUILD_DIR}}/{{APP_NAME}} cmd/nutmix/*.go

# Build recipe
build-dev: gen-proto gen-templ web-build-dev
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building {{APP_NAME}} v{{VERSION}}..."
    mkdir -p {{BUILD_DIR}}
    go build -ldflags="-s -w \
        -X '{{MODULE}}/internal/utils.AppVersion={{VERSION}}' \
        -X '{{MODULE}}/internal/utils.BuildTime={{BUILD_TIME}}' \
        -X '{{MODULE}}/internal/utils.GitCommit={{COMMIT_HASH}}'" \
        -trimpath -o {{BUILD_DIR}}/{{APP_NAME}} cmd/nutmix/*.go

# Dependencies recipe
install-deps:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Checking dependencies..."

    # Install Go tools (pinned versions)
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.0
    go install github.com/a-h/templ/cmd/templ@v0.3.960

    # lint tools
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.6.0

    # Check protobuf-compiler
    if ! command -v protoc >/dev/null 2>&1; then
      echo "Installing protobuf-compiler..."
      if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y protobuf-compiler
      elif command -v brew >/dev/null 2>&1; then
        brew install protobuf
      else
        echo "Please install protobuf-compiler manually for your system"
        exit 1
      fi
    else
      echo "protobuf-compiler already installed"
    fi

    # Download Go module dependencies only if needed
    if [ ! -f go.sum ] || [ go.mod -nt go.sum ]; then
      echo "Running go mod download..."
      go mod download
    else
      echo "Go modules up to date"
    fi

    echo "Installing bun..."
    curl -fsSL https://bun.sh/install | bash

    echo "Installing web dependencies"
    just web-install


    echo "Dependencies check completed"

# Generate protobuf code
gen-proto:
    #!/usr/bin/env bash
    echo "Generating protobuf code..."
    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --experimental_allow_proto3_optional internal/gen/signer.proto

# ============================
# Web Dashaboard
# ============================

# Generate Go code from templ files
gen-templ:
    #!/usr/bin/env bash
    echo "Generating Go code from templ files..."
    templ generate .

# installs all necesary web packages
web-install:
    echo "Intalling npm dependencies"
    cd internal/routes/admin/static 
    bun install

# builds web packages for deployment
web-build-prod:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building web packages (prod, minified)"
    cd internal/routes/admin/static
    mkdir -p dist/js dist/css
    bun build src/index.js --outdir=dist/js --target=browser --format=esm --minify
    cp *.css dist/css/

# builds web packages for local development
web-build-dev:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building web packages (dev, unminified)"
    cd internal/routes/admin/static
    mkdir -p dist/js dist/css
    bun build src/index.js --outdir=dist/js --target=browser --format=esm --sourcemap=inline
    cp *.css dist/css/

# Dev recipe
dev: 
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting development environment..."
    just docker-db
    just run-dev

# Generate test keys
gen-test-keys:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "MINT_PRIVATE_KEY=$(openssl rand -hex 32)"

# Test recipe
test:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running tests..."
    go test -v ./...

# Lint recipe
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running linter..."
    golangci-lint run

# Clean recipe
clean:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Cleaning..."
    rm -rf {{BUILD_DIR}} {{RELEASE_DIR}}

clean-all:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Cleaning..."
    rm -rf {{BUILD_DIR}} {{RELEASE_DIR}}
    # Optionally clean Go caches
    echo "Cleaned Go caches"
    go clean -cache -modcache -testcache

build-platform target:
    #!/usr/bin/env bash
    set -euo pipefail

    # Parse the target (e.g., "linux/amd64")
    OS=$(echo {{target}} | cut -d/ -f1)
    ARCH=$(echo {{target}} | cut -d/ -f2)

    # Set the output binary name
    OUTPUT={{RELEASE_DIR}}/{{APP_NAME}}-${OS}-${ARCH}-{{VERSION}}
    # Note Windows does not build due to syslog not being supported.
    if [ "$OS" = "windows" ]; then
        echo "Warning: Building for Windows is not fully supported due to syslog dependency."
        exit 1
    fi
    if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
        echo "Error: Building for darwin/amd64 is not supported due to dlopen symbol conflict."
        exit 1
    fi

    # Build the binary
    echo "Building for $OS/$ARCH..."
    GOOS=$OS GOARCH=$ARCH go build  \
        -ldflags="-s -w \
        -X '{{MODULE}}/internal/utils.AppVersion={{VERSION}}' \
        -X '{{MODULE}}/internal/utils.BuildTime={{BUILD_TIME}}' \
        -X '{{MODULE}}/internal/utils.GitCommit={{COMMIT_HASH}}'" \
        -trimpath -o $OUTPUT cmd/nutmix/*.go

    echo "Built $OUTPUT"

# Build for all platforms
release: build
    #!/usr/bin/env bash
    set -euo pipefail

    # Clean the release directory
    rm -rf {{RELEASE_DIR}}
    mkdir -p {{RELEASE_DIR}}

    # Build for each platform
    for platform in {{PLATFORMS}}; do
        just build-platform $platform
    done

    echo "Release binaries are in the {{RELEASE_DIR}} directory"

# ============================
# Docker
# ============================

# Docker build recipe
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building Docker image for {{APP_NAME}} v{{VERSION}}..."
    just build
    docker build -t {{DOCKER_IMAGE}}:latest -t {{DOCKER_IMAGE}}:{{VERSION}} .

# Docker run recipe
docker-run:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running {{APP_NAME}} v{{VERSION}} in Docker..."
    just docker-build
    docker run --rm -p 8080:8080 {{DOCKER_IMAGE}}:{{VERSION}}

# Docker push recipe
docker-push:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Pushing Docker images for {{APP_NAME}} v{{VERSION}}..."
    just docker-build
    docker push {{DOCKER_IMAGE}}:latest
    docker push {{DOCKER_IMAGE}}:{{VERSION}}

# Docker clean recipe
docker-clean:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Cleaning up Docker resources..."
    docker rmi -f {{DOCKER_IMAGE}}:latest || true
    docker rmi -f {{DOCKER_IMAGE}}:{{VERSION}} || true

# Docker up recipe
docker-up:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting all services with docker-compose..."
    docker compose up -d

# Docker down recipe
docker-down:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Stopping all services..."
    docker compose down

# Docker db recipe
docker-db:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting database service..."
    docker compose -f docker-compose.yml up -d db
    echo "Waiting for database to be ready..."
    until docker exec nutmix-db pg_isready -U postgrs; do sleep 1; done
    echo "Database is ready!"

# Docker db down recipe
docker-db-down:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Stopping database service..."
    docker compose -f docker-compose.yml down db

# Docker mint recipe
docker-mint:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting mint service..."
    docker compose -f docker-compose.yml -f docker-compose.ports.yml up -d mint

# Docker mint down recipe
docker-mint-down:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Stopping mint service..."
    docker compose -f docker-compose.yml down mint


# ============================
# Versioning
# ============================
# Version recipe
version:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Current version: {{VERSION}}"

version-bump type='patch':
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{type}}" = "major" ]; then
      just version-major
    elif [ "{{type}}" = "minor" ]; then
      just version-minor
    else
      just version-patch
    fi

version-set version:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "{{version}}" > VERSION
    echo "Version set to {{version}}"

version-major:
    #!/usr/bin/env bash
    set -euo pipefail
    # Extract major, minor, and patch from version
    major=$(echo {{VERSION}} | cut -d. -f1)
    minor=$(echo {{VERSION}} | cut -d. -f2)
    patch=$(echo {{VERSION}} | cut -d. -f3)

    # Increment major version and reset minor and patch
    major=$((major + 1))
    minor=0
    patch=0

    # Write new version
    echo "$major.$minor.$patch" > VERSION
    echo "Version bumped to $major.$minor.$patch"

version-minor:
    #!/usr/bin/env bash
    set -euo pipefail
    # Extract major, minor, and patch from version
    major=$(echo {{VERSION}} | cut -d. -f1)
    minor=$(echo {{VERSION}} | cut -d. -f2)
    patch=$(echo {{VERSION}} | cut -d. -f3)

    # Increment minor version and reset patch
    minor=$((minor + 1))
    patch=0

    # Write new version
    echo "$major.$minor.$patch" > VERSION
    echo "Version bumped to $major.$minor.$patch"

version-patch:
    #!/usr/bin/env bash
    set -euo pipefail
    # Extract major, minor, and patch from version
    major=$(echo {{VERSION}} | cut -d. -f1)
    minor=$(echo {{VERSION}} | cut -d. -f2)
    patch=$(echo {{VERSION}} | cut -d. -f3)

    # Increment patch version
    patch=$((patch + 1))

    # Write new version
    echo "$major.$minor.$patch" > VERSION
    echo "Version bumped to $major.$minor.$patch"
