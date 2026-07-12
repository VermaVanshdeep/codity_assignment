// Package logger provides a structured, leveled logger backed by zerolog.
// All application code should use this package rather than importing zerolog directly,
// so that the logging backend can be swapped without touching business logic.
package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger to expose a clean, dependency-injectable interface.
type Logger struct {
	zl zerolog.Logger
}

// Config controls logger initialization.
type Config struct {
	Level  string // debug | info | warn | error
	Format string // json | pretty
}

// New creates and configures a Logger from the given Config.
func New(cfg Config) *Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var out io.Writer = os.Stdout
	if strings.ToLower(cfg.Format) == "pretty" {
		out = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	zl := zerolog.New(out).
		With().
		Timestamp().
		Str("service", "job-scheduler").
		Logger()

	return &Logger{zl: zl}
}

// With returns a child Logger with the given key-value pairs pre-attached.
func (l *Logger) With(key, value string) *Logger {
	return &Logger{zl: l.zl.With().Str(key, value).Logger()}
}

// WithField returns a child Logger with a single string field attached.
func (l *Logger) WithField(key, value string) *Logger {
	return &Logger{zl: l.zl.With().Str(key, value).Logger()}
}

// WithError attaches an error to every subsequent log entry.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{zl: l.zl.With().Err(err).Logger()}
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, fields ...Field) {
	event := l.zl.Debug()
	applyFields(event, fields).Msg(msg)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, fields ...Field) {
	event := l.zl.Info()
	applyFields(event, fields).Msg(msg)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, fields ...Field) {
	event := l.zl.Warn()
	applyFields(event, fields).Msg(msg)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, fields ...Field) {
	event := l.zl.Error()
	applyFields(event, fields).Msg(msg)
}

// Fatal logs a message at FATAL level then calls os.Exit(1).
func (l *Logger) Fatal(msg string, fields ...Field) {
	event := l.zl.Fatal()
	applyFields(event, fields).Msg(msg)
}

// ─── Field Helpers ─────────────────────────────────────────────────────────────

// Field is a single structured log field.
type Field struct {
	Key   string
	Value any
}

// String constructs a string Field.
func String(key, value string) Field { return Field{Key: key, Value: value} }

// Int constructs an int Field.
func Int(key string, value int) Field { return Field{Key: key, Value: value} }

// Int64 constructs an int64 Field.
func Int64(key string, value int64) Field { return Field{Key: key, Value: value} }

// Bool constructs a bool Field.
func Bool(key string, value bool) Field { return Field{Key: key, Value: value} }

// Err constructs an error Field.
func Err(err error) Field { return Field{Key: "error", Value: err} }

// Duration constructs a duration Field (milliseconds).
func Duration(key string, ms int64) Field { return Field{Key: key, Value: ms} }

func applyFields(event *zerolog.Event, fields []Field) *zerolog.Event {
	for _, f := range fields {
		switch v := f.Value.(type) {
		case string:
			event = event.Str(f.Key, v)
		case int:
			event = event.Int(f.Key, v)
		case int64:
			event = event.Int64(f.Key, v)
		case bool:
			event = event.Bool(f.Key, v)
		case error:
			event = event.Err(v)
		default:
			event = event.Interface(f.Key, v)
		}
	}
	return event
}

// Strings constructs a []string Field.
func Strings(key string, value []string) Field { return Field{Key: key, Value: value} }

// Any constructs a generic Field.
func Any(key string, value any) Field { return Field{Key: key, Value: value} }

// Float64 constructs a float64 Field.
func Float64(key string, value float64) Field { return Field{Key: key, Value: value} }
