package logging

import (
	"go.uber.org/zap"
)

var Logger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sugar := logger.Sugar()

	Logger = sugar.WithOptions(zap.AddCallerSkip(1))
}

func Error(msg string, err error) {
	Logger.Errorw(msg, "error", err)
}

func Info(msg string, args ...any) {
	Logger.Infof(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger.Warnf(msg, args...)
}
