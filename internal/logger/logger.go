package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/pulsoats/core/lib/logx"
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

func AsLogx(l zerolog.Logger) logx.Logger {
	return zeroAdapter{l: l}
}

type zeroAdapter struct {
	l zerolog.Logger
}

func (z zeroAdapter) Debug(msg string, kv ...any) { z.log(z.l.Debug(), msg, kv...) }
func (z zeroAdapter) Info(msg string, kv ...any)  { z.log(z.l.Info(), msg, kv...) }
func (z zeroAdapter) Warn(msg string, kv ...any)  { z.log(z.l.Warn(), msg, kv...) }
func (z zeroAdapter) Error(msg string, kv ...any) { z.log(z.l.Error(), msg, kv...) }

func (z zeroAdapter) log(e *zerolog.Event, msg string, kv ...any) {
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		v := kv[i+1]
		if err, ok := v.(error); ok {
			// Важный момент: e.Err(err) пишет в поле "error".
			// Если ты хочешь именно key=k, можно сделать e.Str(k, err.Error()).
			e.Err(err)
			continue
		}
		e.Interface(k, v)
	}
	e.Msg(msg)
}
