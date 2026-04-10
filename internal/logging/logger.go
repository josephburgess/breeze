package logging

import (
	"fmt"
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

func Error(msg string, err error) {
	logger.Error(msg, "error", err)
}

func Info(msg string, args ...any) {
	if len(args) == 0 {
		logger.Info(msg)
		return
	}
	logger.Info(fmt.Sprintf(msg, args...))
}

func Warn(msg string, args ...any) {
	if len(args) == 0 {
		logger.Warn(msg)
		return
	}
	logger.Warn(fmt.Sprintf(msg, args...))
}
