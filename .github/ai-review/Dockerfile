FROM rust:1.98.1-bookworm@sha256:93ce27a88655056a51dbdd8f5f2d7ddc071c7b0070fb288a37b5a285fc83971e AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 git pkg-config cmake protobuf-compiler libssl-dev libsqlite3-dev zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /integration
COPY fetch.py upstream.json export.rs boundary_tests.rs ./
COPY patches/*.patch ./patches/
RUN python3 fetch.py /source
WORKDIR /source
ENV CARGO_BUILD_JOBS=2 CARGO_PROFILE_DEV_DEBUG=0 CARGO_PROFILE_RELEASE_STRIP=true
# The overlay changes no dependency pins or lockfile entries.
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    --mount=type=cache,target=/source/target \
    cargo test --locked --bin nutmix-export --bin nutmix-boundary-tests && \
    cargo test --locked --lib reporting::tests && \
    cargo test --locked --lib inspection::tests && \
    cargo build --locked --release --bin fiach --bin nutmix-export && \
    mkdir /binaries && cp target/release/fiach target/release/nutmix-export /binaries/
COPY mock_test.py fiach.toml /integration/
COPY personas/ /integration/personas/
COPY skills/ /integration/skills/
RUN python3 /integration/mock_test.py --binary /binaries/fiach --exporter /binaries/nutmix-export --source /source

FROM golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS go

FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git gh ripgrep \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /binaries/ /usr/local/bin/
COPY --from=go /usr/local/go /usr/local/go
COPY --from=build /source/LICENSE-MIT /source/LICENSE-APACHE /usr/share/doc/fiach/
COPY fiach.toml upstream.json /etc/nutmix-review/
COPY personas/ /etc/nutmix-review/personas/
COPY skills/ /etc/fiach/skills/
ENV PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin HOME=/tmp/home \
    XDG_CONFIG_HOME=/tmp/config XDG_STATE_HOME=/tmp/state GOOSE_PATH_ROOT=/tmp/goose \
    GOTOOLCHAIN=local GOMODCACHE=/tmp/gomod GOCACHE=/tmp/gocache GOOSE_MODE=auto
WORKDIR /etc/nutmix-review
USER 10001:10001
ENTRYPOINT ["fiach"]
