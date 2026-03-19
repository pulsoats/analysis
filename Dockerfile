# syntax=docker/dockerfile:1.6

########################
# Builder
########################
FROM golang:1.25.3 AS builder

WORKDIR /src

# зависимости и vendor
COPY go.mod go.sum ./
COPY vendor ./vendor

# код
COPY . .

# сборка через vendor
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -mod=vendor -o /out/analysis-service ./cmd

########################
# Runtime
########################
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

# non-root user
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