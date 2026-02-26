package logx

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(LevelInfo))
}

func ParseLevel(v string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level %q (expected debug|info|warn|error)", v)
	}
}

func SetLevel(v string) error {
	lvl, err := ParseLevel(v)
	if err != nil {
		return err
	}
	currentLevel.Store(int32(lvl))
	return nil
}

// Configure resolves log level from flags and env.
// Precedence: --log-level > --verbose > JINGUI_LOG_LEVEL > default(info).
func Configure(flagLevel string, verbose bool) error {
	if strings.TrimSpace(flagLevel) != "" {
		return SetLevel(flagLevel)
	}
	if verbose {
		return SetLevel("debug")
	}
	if env := strings.TrimSpace(os.Getenv("JINGUI_LOG_LEVEL")); env != "" {
		return SetLevel(env)
	}
	return SetLevel("info")
}

func levelEnabled(l Level) bool {
	return l >= Level(currentLevel.Load())
}

func IsDebug() bool {
	return levelEnabled(LevelDebug)
}

func logf(l Level, label, format string, args ...any) {
	if !levelEnabled(l) {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s [%s] %s\n", ts, label, msg)
}

func Debugf(format string, args ...any) { logf(LevelDebug, "DEBUG", format, args...) }
func Infof(format string, args ...any)  { logf(LevelInfo, "INFO", format, args...) }
func Warnf(format string, args ...any)  { logf(LevelWarn, "WARN", format, args...) }
func Errorf(format string, args ...any) { logf(LevelError, "ERROR", format, args...) }
