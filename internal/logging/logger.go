package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/Marlliton/slogpretty"
)

func NewLogger(logLevel string, pretty bool) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(strings.ToLower(logLevel))); err != nil {
		level = slog.LevelInfo
	}

	if pretty {
		handler := slogpretty.New(os.Stdout, nil)
		slog.SetDefault(slog.New(handler))

		return slog.New(handler)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	})

	return slog.New(handler)
}
