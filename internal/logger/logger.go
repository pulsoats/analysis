package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	logFormatJSON    = "json"
	logFormatConsole = "console"
)

// Configure sets up the global zerolog logger according to environment variables
// LOG_LEVEL (debug, info, warn, error) and LOG_FORMAT (json, console).
func Configure() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	level := parseLevel(os.Getenv("LOG_LEVEL"))
	writer := selectWriter(os.Getenv("LOG_FORMAT"))

	logger := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	log.Logger = logger
	return logger
}

// NewSlogLogger wraps a zerolog.Logger into a *slog.Logger so it can be passed
// to components that follow the core library convention of accepting *slog.Logger.
func NewSlogLogger(l zerolog.Logger) *slog.Logger {
	return slog.New(&zerologSlogHandler{l: l})
}

func parseLevel(value string) zerolog.Level {
	if value == "" {
		return zerolog.InfoLevel
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(value))
	if err != nil {
		return zerolog.InfoLevel
	}
	return lvl
}

func selectWriter(format string) io.Writer {
	if strings.EqualFold(format, logFormatJSON) {
		return os.Stdout
	}

	return zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
}

// zerologSlogHandler implements slog.Handler on top of zerolog.Logger.
// It delegates all structured logging to the underlying zerolog.Logger so that
// components receiving a *slog.Logger still write through zerolog.
type zerologSlogHandler struct {
	l      zerolog.Logger
	prefix string // dot-separated group prefix, e.g. "request."
}

func (h *zerologSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.l.GetLevel() <= zlLevel(level)
}

func (h *zerologSlogHandler) Handle(_ context.Context, r slog.Record) error {
	e := h.event(r.Level)
	if e == nil {
		return nil
	}
	r.Attrs(func(a slog.Attr) bool {
		h.appendAttr(e, a)
		return true
	})
	e.Msg(r.Message)
	return nil
}

func (h *zerologSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	ctx := h.l.With()
	for _, a := range attrs {
		addAttrToCtx(&ctx, h.prefix, a)
	}
	return &zerologSlogHandler{l: ctx.Logger(), prefix: h.prefix}
}

func (h *zerologSlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &zerologSlogHandler{l: h.l, prefix: h.prefix + name + "."}
}

func (h *zerologSlogHandler) event(level slog.Level) *zerolog.Event {
	switch {
	case level >= slog.LevelError:
		return h.l.Error()
	case level >= slog.LevelWarn:
		return h.l.Warn()
	case level >= slog.LevelInfo:
		return h.l.Info()
	default:
		return h.l.Debug()
	}
}

func (h *zerologSlogHandler) appendAttr(e *zerolog.Event, a slog.Attr) {
	a.Value = a.Value.Resolve()
	key := h.prefix + a.Key
	if a.Value.Kind() == slog.KindGroup {
		sub := &zerologSlogHandler{l: h.l, prefix: key + "."}
		for _, ga := range a.Value.Group() {
			sub.appendAttr(e, ga)
		}
		return
	}
	if err, ok := a.Value.Any().(error); ok {
		e.Err(err)
		return
	}
	e.Interface(key, a.Value.Any())
}

func addAttrToCtx(ctx *zerolog.Context, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	key := prefix + a.Key
	if a.Value.Kind() == slog.KindGroup {
		for _, sub := range a.Value.Group() {
			addAttrToCtx(ctx, key+".", sub)
		}
		return
	}
	*ctx = ctx.Interface(key, a.Value.Any())
}

func zlLevel(level slog.Level) zerolog.Level {
	switch {
	case level >= slog.LevelError:
		return zerolog.ErrorLevel
	case level >= slog.LevelWarn:
		return zerolog.WarnLevel
	case level >= slog.LevelInfo:
		return zerolog.InfoLevel
	default:
		return zerolog.DebugLevel
	}
}
