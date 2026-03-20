# syntax=docker/dockerfile:1.6

FROM golang:1.25.3 AS builder

ARG GITHUB_TOKEN
ENV GOPRIVATE=github.com/pulsoats/* \
    GONOSUMDB=github.com/pulsoats/* \
    GONOPROXY=github.com/pulsoats/*

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    set -e; \
    if [ -n "${GITHUB_TOKEN:-}" ]; then \
        printf "machine github.com\nlogin %s\npassword x-oauth-basic\n" "$GITHUB_TOKEN" > /root/.netrc; \
        chmod 600 /root/.netrc; \
    fi; \
    go mod download; \
    rm -f /root/.netrc

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    set -e; \
    mkdir -p /out; \
    if [ -n "${GITHUB_TOKEN:-}" ]; then \
        printf "machine github.com\nlogin %s\npassword x-oauth-basic\n" "$GITHUB_TOKEN" > /root/.netrc; \
        chmod 600 /root/.netrc; \
    fi; \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/analysis-service ./cmd; \
    rm -f /root/.netrc

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -r -u 10001 analysis \
    && mkdir -p /data \
    && chown -R analysis:analysis /data

WORKDIR /app

COPY --from=builder /out/analysis-service /usr/local/bin/analysis-service

ENV ANALYSIS_STORAGE_DIR=/data/runs \
    ANALYSIS_GRPC_ADDR=:50051

EXPOSE 50051
VOLUME ["/data"]

USER analysis
ENTRYPOINT ["/usr/local/bin/analysis-service"]