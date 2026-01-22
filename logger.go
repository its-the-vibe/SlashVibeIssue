package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	currentLogLevel LogLevel = INFO
	logLevelNames            = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}
)

// SetLogLevel sets the minimum log level for output
func SetLogLevel(level string) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		currentLogLevel = DEBUG
	case "INFO":
		currentLogLevel = INFO
	case "WARN":
		currentLogLevel = WARN
	case "ERROR":
		currentLogLevel = ERROR
	default:
		// Use custom logger for consistency
		currentLogLevel = INFO
		logf(WARN, "Unknown log level '%s', defaulting to INFO", level)
	}
}

// logf logs a formatted message at the specified level
func logf(level LogLevel, format string, args ...interface{}) {
	if level >= currentLogLevel {
		prefix := fmt.Sprintf("[%s] ", logLevelNames[level])
		log.Printf(prefix+format, args...)
	}
}

// logln logs a message at the specified level
func logln(level LogLevel, args ...interface{}) {
	if level >= currentLogLevel {
		prefix := fmt.Sprintf("[%s] ", logLevelNames[level])
		msg := fmt.Sprint(args...)
		log.Println(prefix + msg)
	}
}

// Debug logs a debug message (most verbose)
func Debug(format string, args ...interface{}) {
	logf(DEBUG, format, args...)
}

// Info logs an informational message
func Info(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	logf(WARN, format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	logf(ERROR, format, args...)
}

// Fatal logs a fatal error and exits
func Fatal(format string, args ...interface{}) {
	logf(ERROR, format, args...)
	os.Exit(1)
}
