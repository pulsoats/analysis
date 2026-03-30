---
name: Logging architecture
description: How logging is set up in the analysis service — zerolog backend, slog injection pattern matching core library
type: project
---

Zerolog is the logging backend, but components receive `*slog.Logger` (matching core library convention).

**Setup:**
- `internal/logger/logger.go` — `Configure()` sets up global zerolog; `NewSlogLogger(zerolog.Logger) *slog.Logger` bridges zerolog → slog via a custom `slog.Handler` (`zerologSlogHandler`)
- `cmd/main.go` — creates one `slogLogger` and passes it to all components

**Injection pattern (functional options, same as core):**
- `exchanges.NewRegistry(exchanges.WithLogger(slogLogger))` — core library, expects `*slog.Logger`
- `run.ServiceConfig{Logger: slogLogger}` — service layer, config struct field
- `transportgrpc.NewAnalysisServer(svc, transportgrpc.WithLogger(slogLogger))` — transport layer, functional option
- `transportgrpc.RunGRPCServer(ctx, addr, srv, slogLogger)` — direct parameter

**Component labeling:** each component adds `"component"` key via `.With("component", "...")`.

**Why:** core library (`github.com/pulsoats/core`) uses `*slog.Logger` with functional options and `logx.Discard()` as default. Analysis matches this to be consistent and avoid a custom adapter type that no longer exists in core v1.2.0.

**How to apply:** when adding new services/repositories, inject `*slog.Logger` via config struct field or `WithLogger` functional option; default to `logx.Discard()` if nil.
