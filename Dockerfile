# syntax=docker/dockerfile:1.6

FROM golang:1.25.3 AS builder

ENV GOPRIVATE=github.com/pulsoats/* \
    GONOSUMDB=github.com/pulsoats/* \
    GONOPROXY=github.com/pulsoats/*

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=github_token \
    set -e; \
    if [ -f /run/secrets/github_token ]; then \
        printf "machine github.com\nlogin %s\npassword x-oauth-basic\n" \
            "$(cat /run/secrets/github_token)" > /root/.netrc; \
        chmod 600 /root/.netrc; \
    fi; \
    go mod download; \
    rm -f /root/.netrc

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=github_token \
    set -e; \
    mkdir -p /out; \
    if [ -f /run/secrets/github_token ]; then \
        printf "machine github.com\nlogin %s\npassword x-oauth-basic\n" \
            "$(cat /run/secrets/github_token)" > /root/.netrc; \
        chmod 600 /root/.netrc; \
    fi; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags "-X main.version=$(git describe --tags --always --dirty)" \
        -o /out/analysis-app ./cmd/analysis; \
    rm -f /root/.netrc

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -r -u 10001 analysis \
    && mkdir -p /data \
    && chown -R analysis:analysis /data

WORKDIR /app

COPY --from=builder /out/analysis-app /usr/local/bin/analysis-app

VOLUME ["/data"]

USER analysis
ENTRYPOINT ["/usr/local/bin/analysis-app"]