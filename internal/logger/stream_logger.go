package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

// StreamLogger handles per-stream logging with line rotation
type StreamLogger struct {
	mu       sync.Mutex
	filePath string
	maxLines int
}

// NewStreamLogger creates a logger for a specific stream
func NewStreamLogger(dataDir, streamName string, maxLines int) *StreamLogger {
	if maxLines <= 0 {
		maxLines = 100
	}
	return &StreamLogger{
		filePath: filepath.Join(dataDir, streamName+".log"),
		maxLines: maxLines,
	}
}

// Log writes a message with the specified level
func (l *StreamLogger) Log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	// Append to file
	f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()

	// Rotate if needed
	l.rotate()
}

// Info logs an info-level message
func (l *StreamLogger) Info(format string, args ...interface{}) {
	l.Log(LevelInfo, format, args...)
}

// Warn logs a warning-level message
func (l *StreamLogger) Warn(format string, args ...interface{}) {
	l.Log(LevelWarn, format, args...)
}

// Error logs an error-level message
func (l *StreamLogger) Error(format string, args ...interface{}) {
	l.Log(LevelError, format, args...)
}

// rotate keeps only the last maxLines in the log file
func (l *StreamLogger) rotate() {
	// Read all lines
	content, err := os.ReadFile(l.filePath)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")

	// Remove empty trailing line from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Only rotate if exceeds maxLines
	if len(lines) <= l.maxLines {
		return
	}

	// Keep only the last maxLines
	lines = lines[len(lines)-l.maxLines:]

	// Write back
	f, err := os.Create(l.filePath)
	if err != nil {
		return
	}
	defer f.Close()

	for _, line := range lines {
		f.WriteString(line + "\n")
	}
}

// GetPath returns the log file path
func (l *StreamLogger) GetPath() string {
	return l.filePath
}

// ReadLast reads the last n lines from the log file
func (l *StreamLogger) ReadLast(n int) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if n >= len(lines) {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

// LoggerManager manages loggers for multiple streams
type LoggerManager struct {
	mu      sync.RWMutex
	loggers map[string]*StreamLogger
	dataDir string
	maxLines int
}

// NewLoggerManager creates a new logger manager
func NewLoggerManager(dataDir string, maxLines int) *LoggerManager {
	return &LoggerManager{
		loggers:  make(map[string]*StreamLogger),
		dataDir:  dataDir,
		maxLines: maxLines,
	}
}

// GetLogger returns (or creates) a logger for the given stream
func (m *LoggerManager) GetLogger(streamName string) *StreamLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	if logger, exists := m.loggers[streamName]; exists {
		return logger
	}

	logger := NewStreamLogger(m.dataDir, streamName, m.maxLines)
	m.loggers[streamName] = logger
	return logger
}

// RemoveLogger removes a logger from the manager (does not delete the file)
func (m *LoggerManager) RemoveLogger(streamName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.loggers, streamName)
}
