// Package log configures the structured application logger.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// New opens filePath for appending and returns a JSON logger writing to it.
// The returned closer must be closed on shutdown.
func New(filePath string) (*slog.Logger, io.Closer, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return logger, file, nil
}
