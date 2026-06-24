// Package logger provides a configured slog logger for the application.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
)

const LogLevel = "LOG_LEVEL"

var sourceLogger *slog.Logger
var defaultLogger *slog.Logger

var level = new(slog.LevelVar)

type sourceHandler struct {
	slog.Handler
}

func (h *sourceHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.PC != 0 {
		var pcs [1]uintptr
		runtime.Callers(5, pcs[:])
		r.PC = pcs[0]
	}
	return h.Handler.Handle(ctx, r)
}

func init() {
	sourceHandler := &sourceHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	})}
	defaultHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	sourceLogger = slog.New(sourceHandler)
	defaultLogger = slog.New(defaultHandler)

	setLogLevel()
}

func setLogLevel() {
	envLogLevel, found := os.LookupEnv(LogLevel)
	if !found {
		envLogLevel = "info"
	} else {
		envLogLevel = strings.ToLower(envLogLevel)
	}

	switch envLogLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "info":
		level.Set(slog.LevelInfo)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
}

func Debug(msg string, args ...any) {
	sourceLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	sourceLogger.Error(msg, args...)
}

func Fatal(msg string, args ...any) error {
	sourceLogger.Error(msg, args...)
	return fmt.Errorf("%s %v", msg, args)
}
