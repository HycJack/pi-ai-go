package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel 表示日志条目的严重级别。
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

// Logger 是一个轻量级文件日志器，按天滚动。
type Logger struct {
	mu      sync.Mutex
	logDir  string
	file    *os.File
	curDate string
	level   LogLevel
}

// NewLogger 创建一个写入 logDir 的 Logger。
func NewLogger(logDir string, level LogLevel) *Logger {
	l := &Logger{logDir: logDir, level: level}
	_ = os.MkdirAll(logDir, 0o755)
	l.rotate()
	return l
}

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
		fmt.Fprintf(os.Stderr, "logger: cannot open %s: %v\n", path, err)
		return
	}
	l.file = f
	l.curDate = today
}

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

func (l *Logger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.log(LevelInfo, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.log(LevelWarn, format, args...) }
func (l *Logger) Error(format string, args ...any) { l.log(LevelError, format, args...) }

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

var StdLogger *Logger

// InitLogger 初始化全局日志器，日志写入 ~/.handwriting-desktop/logs。
func InitLogger(homeDir string) {
	if StdLogger != nil {
		return
	}
	logDir := filepath.Join(homeDir, ".handwriting-desktop", "logs")
	StdLogger = NewLogger(logDir, LevelDebug)
}

func LogInfo(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Info(format, args...)
	}
}

func LogWarn(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Warn(format, args...)
	}
}

func LogError(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Error(format, args...)
	}
}

func LogDebug(format string, args ...any) {
	if StdLogger != nil {
		StdLogger.Debug(format, args...)
	}
}
