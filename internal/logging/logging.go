package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gitlab.digital-spirit.ru/solutions/common/kan/internal/config"
)

func Open(path, level string) (*slog.Logger, io.Closer, error) {
	if err := config.EnsureParent(path); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log %q: %w", path, err)
	}

	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info", "":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		file.Close()
		return nil, nil, fmt.Errorf("invalid log level %q", level)
	}

	logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slogLevel}))
	return logger, file, nil
}
