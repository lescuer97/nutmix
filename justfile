# Environment variables
export PATH := env_var("PATH") + ":" + `go env GOPATH` + "/bin"

# Variables
APP_NAME := "nutmix"
DOCKER_IMAGE := "nutmix"
BUILD_DIR := "build"
CGO_ENABLED := "0"
GOOS := "linux"
GOARCH := "amd64"
RUN_ARGS := ""  # Additional arguments for running the app locally
# Read current version from VERSION file
VERSION := `cat VERSION 2>/dev/null || echo "0.0.0"`
GO_BIN := `go env GOPATH`
PROTOC_GEN_GO := `which protoc-gen-go || echo "$(go env GOPATH)/bin/protoc-gen-go"`
PROTOC_GEN_GO_GRPC := `which protoc-gen-go-grpc || echo "$(go env GOPATH)/bin/protoc-gen-go-grpc"`
TEMPL_CMD := `which templ || echo "$(go env GOPATH)/bin/templ"`


# Default task
default:
    @just --list

# Help task
help:
    @echo "Available tasks:"
    @echo "  build            - Build the application"
    @echo "  run              - Build and run the application locally"
    @echo "  dev              - Start database and run application locally"
    @echo "  deps             - Install required dependencies"
    @echo "  gen-proto        - Generate protobuf code"
    @echo "  gen-templ        - Generate go code from templ files"
    @echo "  gen-test-keys    - Generate test keys for MINT_PRIVATE_KEY"
    @echo "  test             - Run tests"
    @echo "  lint             - Run linter"
    @echo "  clean            - Clean build artifacts"
    @echo "  version          - Show current version"
    @echo "  version-bump     - Bump version (patch by default)"
    @echo "  version-set      - Set specific version (e.g., just version-set 1.2.3)"
    @echo "  version-major    - Bump major version"
    @echo "  version-minor    - Bump minor version"
    @echo "  version-patch    - Bump patch version"
    @echo "  docker-build     - Build Docker image with version tags"
    @echo "  docker-run       - Run application in Docker"
    @echo "  docker-push      - Push Docker images to registry"
    @echo "  docker-clean     - Clean up Docker resources"
    @echo "  docker-up        - Start all services with docker-compose"
    @echo "  docker-down      - Stop all services"
    @echo "  docker-db        - Start only the database service"
    @echo "  docker-db-down   - Stop the database service"

# Build task
build:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building {{APP_NAME}} v{{VERSION}}..."
    mkdir -p {{BUILD_DIR}}
    just gen-proto
    just gen-templ
    go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o {{BUILD_DIR}}/{{APP_NAME}} cmd/nutmix/*.go

# Dependencies task
deps:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Checking dependencies..."

    # Helper to check and install Go tools
    install_if_missing() {
      local tool=$1
      local pkg=$2
      if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Installing $tool..."
        go install "$pkg"
      else
        echo "$tool already installed"
      fi
    }

    # Check Go tools (pinned versions)
    install_if_missing protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.1
    install_if_missing protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
    install_if_missing goose github.com/pressly/goose/v3/cmd/goose@v3.21.1
    install_if_missing templ github.com/a-h/templ/cmd/templ@v0.2.747

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

    echo "Dependencies check completed"

# Generate protobuf code
gen-proto:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating protobuf code..."
    just deps
    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --experimental_allow_proto3_optional internal/gen/signer.proto

# Generate Go code from templ files
gen-templ:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating Go code from templ files..."
    just deps
    templ generate internal/routes/admin/templates/

# Run task
run:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running {{APP_NAME}} v{{VERSION}} locally..."
    just build
    ./{{BUILD_DIR}}/{{APP_NAME}} {{RUN_ARGS}}

# Dev task
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting development environment..."
    just docker-db
    DATABASE_URL="postgres://nutmix:admin@localhost:5432/nutmix"
    POSTGRES_HOST="localhost"
    POSTGRES_PORT="5432"
    just run

# Generate test keys
gen-test-keys:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "MINT_PRIVATE_KEY=$(openssl rand -hex 32)"
    #echo "use the npub generated below for ADMIN_NOSTR_NPUB"
    #go run ./cmd/gen_nostr_key

# Test task
test:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running tests..."
    go test -v ./...

# Lint task
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running linter..."
    golangci-lint run

# Clean task
clean:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Cleaning..."
    rm -rf {{BUILD_DIR}}

# Docker build task
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building Docker image for {{APP_NAME}} v{{VERSION}}..."
    just build
    docker build -t {{DOCKER_IMAGE}}:latest -t {{DOCKER_IMAGE}}:{{VERSION}} .

# Docker run task
docker-run:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running {{APP_NAME}} v{{VERSION}} in Docker..."
    just docker-build
    docker run --rm -p 8080:8080 {{DOCKER_IMAGE}}:{{VERSION}}

# Docker push task
docker-push:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Pushing Docker images for {{APP_NAME}} v{{VERSION}}..."
    just docker-build
    docker push {{DOCKER_IMAGE}}:latest
    docker push {{DOCKER_IMAGE}}:{{VERSION}}

# Docker clean task
docker-clean:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Cleaning up Docker resources..."
    docker rmi -f {{DOCKER_IMAGE}}:latest || true
    docker rmi -f {{DOCKER_IMAGE}}:{{VERSION}} || true

# Docker up task
docker-up:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting all services with docker-compose..."
    export UID=$(id -u) && export GID=$(id -g) && docker compose up -d

# Docker down task
docker-down:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Stopping all services..."
    docker compose down

# Docker db task
docker-db:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting database service..."
    docker compose -f docker-compose-dev.yml up -d db

# Docker db down task
docker-db-down:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Stopping database service..."
    docker compose -f docker-compose-dev.yml down db

# Version tasks
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
