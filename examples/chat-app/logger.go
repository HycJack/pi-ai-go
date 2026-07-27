package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	}
	return "?"
}

// Logger is a lightweight file logger that writes to a daily-rotated
// log file under <logDir>/app-YYYY-MM-DD.log. It is safe for
// concurrent use.
type Logger struct {
	mu      sync.Mutex
	logDir  string
	file    *os.File
	curDate string
	level   LogLevel
}

// NewLogger creates a Logger that writes to logDir. The directory is
// created if it does not exist. Files rotate daily.
func NewLogger(logDir string, level LogLevel) *Logger {
	l := &Logger{logDir: logDir, level: level}
	_ = os.MkdirAll(logDir, 0o755)
	l.rotate() // open today's file immediately
	return l
}

// rotate opens a new file for the current date if needed.
func (l *Logger) rotate() {
	today := time.Now().Format("2006-01-02")
	if l.file != nil && today == l.curDate {
		return
	}
	if l.file != nil {
		l.file.Close()
	}
	path := filepath.Join(l.logDir, fmt.Sprintf("app-%s.log", today))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// Fallback to stderr so we at least see the error somewhere.
		fmt.Fprintf(os.Stderr, "logger: cannot open %s: %v\n", path, err)
		return
	}
	l.file = f
	l.curDate = today
}

// log writes a formatted line to the current log file.
func (l *Logger) log(level LogLevel, format string, args ...any) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rotate()
	if l.file == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "%s [%s] %s\n", ts, level, msg)
}

// Debug logs a debug-level message.
func (l *Logger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }

// Info logs an info-level message.
func (l *Logger) Info(format string, args ...any) { l.log(LevelInfo, format, args...) }

// Warn logs a warning-level message.
func (l *Logger) Warn(format string, args ...any) { l.log(LevelWarn, format, args...) }

// Error logs an error-level message.
func (l *Logger) Error(format string, args ...any) { l.log(LevelError, format, args...) }

// Write implements io.Writer so the Logger can be used as a log sink
// for libraries that expect an io.Writer (e.g. standard log package).
// It returns the number of bytes written, or 0 if the log level filters
// the entry out (per io.Writer contract).
func (l *Logger) Write(p []byte) (int, error) {
	if LevelInfo < l.level {
		return 0, nil
	}
	l.log(LevelInfo, "%s", string(p))
	return len(p), nil
}

// Close closes the current log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// StdLogger is a package-level logger instance, initialized in
// InitLogger. It is nil-safe: calling its methods when nil is a no-op
// (handled by the app code that checks for nil).
var StdLogger *Logger

// InitLogger initializes the package-level StdLogger to write to
// <homeDir>/.pi-chat-app/logs/. It is safe to call multiple times;
// only the first call has effect.
func InitLogger(homeDir string) {
	if StdLogger != nil {
		return
	}
	logDir := filepath.Join(homeDir, ".pi-chat-app", "logs")
	StdLogger = NewLogger(logDir, LevelDebug)
}

// LogInfo is a convenience wrapper that is nil-safe.
func LogInfo(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Info(format, args...)
	}
}

// LogWarn is a convenience wrapper that is nil-safe.
func LogWarn(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Warn(format, args...)
	}
}

// LogError is a convenience wrapper that is nil-safe.
func LogError(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Error(format, args...)
	}
}

// LogDebug is a convenience wrapper that is nil-safe.
func LogDebug(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Debug(format, args...)
	}
}

// Ensure Logger satisfies io.Writer at compile time.
var _ io.Writer = (*Logger)(nil)
