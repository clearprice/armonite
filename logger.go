package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	level  LogLevel
	format string
	writer io.Writer
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
}

var globalLogger *Logger

func InitLogger(config LoggingConfig) (*Logger, error) {
	level := parseLogLevel(config.Level)
	format := strings.ToLower(config.Format)

	var writer io.Writer = os.Stdout
	if config.File != "" {
		file, err := os.OpenFile(config.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", config.File, err)
		}
		writer = io.MultiWriter(os.Stdout, file)
	}

	logger := &Logger{
		level:  level,
		format: format,
		writer: writer,
	}

	globalLogger = logger
	return logger, nil
}

func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}

func (l *Logger) log(level LogLevel, message string) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
	}

	// Add file and line info for debug and error levels
	if level == DEBUG || level == ERROR || level == FATAL {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			// Get just the filename, not the full path
			parts := strings.Split(file, "/")
			entry.File = parts[len(parts)-1]
			entry.Line = line
		}
	}

	var output string
	if l.format == "json" {
		jsonData, _ := json.Marshal(entry)
		output = string(jsonData)
	} else {
		// Text format
		if entry.File != "" {
			output = fmt.Sprintf("[%s] %s %s (%s:%d)",
				entry.Timestamp, entry.Level, entry.Message, entry.File, entry.Line)
		} else {
			output = fmt.Sprintf("[%s] %s %s",
				entry.Timestamp, entry.Level, entry.Message)
		}
	}

	fmt.Fprintln(l.writer, output)

	// Exit on fatal
	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(msg, args...))
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(msg, args...))
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(msg, args...))
}

func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(msg, args...))
}

// Global logger functions
func GetLogger() *Logger {
	if globalLogger == nil {
		config := LoggingConfig{Level: "info", Format: "text"}
		logger, _ := InitLogger(config)
		return logger
	}
	return globalLogger
}

func LogDebug(msg string, args ...interface{}) {
	GetLogger().Debug(msg, args...)
}

func LogInfo(msg string, args ...interface{}) {
	GetLogger().Info(msg, args...)
}

func LogWarn(msg string, args ...interface{}) {
	GetLogger().Warn(msg, args...)
}

func LogError(msg string, args ...interface{}) {
	GetLogger().Error(msg, args...)
}

func LogFatal(msg string, args ...interface{}) {
	GetLogger().Fatal(msg, args...)
}

// Replace standard log package usage
func SetupStandardLogger() {
	log.SetOutput(io.Discard) // Disable standard log output
}
