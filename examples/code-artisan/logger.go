package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger provides simple file logging.
type Logger struct {
	file *os.File
}

var StdLogger *Logger

func InitLogger(baseDir string) {
	logDir := filepath.Join(baseDir, ".code-artisan", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		StdLogger = &Logger{}
		return
	}

	logFile := filepath.Join(logDir, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		StdLogger = &Logger{}
		return
	}

	StdLogger = &Logger{file: f}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l == nil || l.file == nil {
		return
	}
	log.Printf("[DEBUG] "+format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l == nil || l.file == nil {
		return
	}
	log.Printf("[INFO] "+format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l == nil || l.file == nil {
		return
	}
	log.Printf("[WARN] "+format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l == nil || l.file == nil {
		return
	}
	log.Printf("[ERROR] "+format, args...)
}

func LogInfo(format string, args ...interface{}) {
	if StdLogger != nil {
		StdLogger.Info(format, args...)
	}
}

func LogError(format string, args ...interface{}) {
	if StdLogger != nil {
		StdLogger.Error(format, args...)
	}
}

func LogWarn(format string, args ...interface{}) {
	if StdLogger != nil {
		StdLogger.Warn(format, args...)
	}
}

func LogDebug(format string, args ...interface{}) {
	if StdLogger != nil {
		StdLogger.Debug(format, args...)
	}
}
