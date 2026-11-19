package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger provides a centralized logging system for GotoSplitter
type Logger struct {
	fileLogger *log.Logger
	mu         sync.Mutex
	lastLog    map[string]time.Time
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger with timestamped log file in user config directory
func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "gotosplitter")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create timestamped log file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(configDir, fmt.Sprintf("gotosplitter_%s.log", timestamp))
	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Create the logger
	globalLogger = &Logger{
		fileLogger: log.New(file, "", log.LstdFlags|log.Lshortfile),
		lastLog:    make(map[string]time.Time),
	}

	// Clean up old log files (keep only last 5)
	go cleanupOldLogs(configDir)

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// If logger wasn't initialized, create a simple one that logs to stderr
		return &Logger{
			fileLogger: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
			lastLog:    make(map[string]time.Time),
		}
	}
	return globalLogger
}

// cleanupOldLogs removes old log files, keeping only the last 5
func cleanupOldLogs(configDir string) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return
	}

	var logFiles []os.DirEntry
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "gotosplitter_") && strings.HasSuffix(entry.Name(), ".log") {
			logFiles = append(logFiles, entry)
		}
	}

	// Sort by modification time (oldest first)
	if len(logFiles) <= 5 {
		return
	}

	// Sort by modification time - we want to remove oldest ones
	for i := 0; i < len(logFiles)-5; i++ {
		entry := logFiles[i]
		filePath := filepath.Join(configDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			// Log error but don't fail the whole process
			log.Printf("warning: failed to remove old log file %s: %v", filePath, err)
		}
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.logWithDeduplication("INFO", format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.logWithDeduplication("WARN", format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.logWithDeduplication("ERROR", format, v...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.logWithDeduplication("DEBUG", format, v...)
}

// logWithDeduplication logs a message only if it's not been logged recently or if it's different from the last one
func (l *Logger) logWithDeduplication(level, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := fmt.Sprintf(format, v...)
	key := fmt.Sprintf("[%s] %s", level, message)

	// Check if this exact message was logged recently (within 10 seconds)
	if lastTime, exists := l.lastLog[key]; exists {
		if time.Since(lastTime) < 10*time.Second {
			return // Skip logging this message
		}
	}

	// Update the last log time for this specific message
	l.lastLog[key] = time.Now()

	// Log to file
	l.fileLogger.Printf("%s: %s", level, message)
}
