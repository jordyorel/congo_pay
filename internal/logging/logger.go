package logging

import (
	"io"
	"log/slog"
	"os"
)

// New creates a JSON slog logger configured at the provided level. If the
// level string is invalid it defaults to info.
func New(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl.Set(slog.LevelInfo)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}

// Discard returns a logger that drops all output. Useful for tests.
func Discard() *slog.Logger {
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})
	return slog.New(handler)
}
