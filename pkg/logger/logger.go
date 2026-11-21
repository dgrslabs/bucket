package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dgrslabs/bucket/internal/domain"
)

type BucketLogger struct {
	Logger  *log.Logger
	silence bool
}

type LogFunc func(domain.BucketLogger)

func NewLogger(logFile string, logTerminal, silence bool, opts ...LogFunc) domain.BucketLogger {
	bucketLogger := &BucketLogger{silence: silence}

	for _, opt := range opts {
		opt(bucketLogger)
	}

	if !logTerminal && logFile == "" {
		logTerminal = true
	}

	var logWriter io.Writer

	// Setup log file if provided
	if logFile != "" {
		logDir := filepath.Join(logFile, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatal("Failed to create log directory:", err)
		}

		infoLogFile, err := os.OpenFile(filepath.Join(logDir, "info.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Failed to open info log file:", err)
		}

		if logTerminal && !silence {
			logWriter = io.MultiWriter(os.Stdout, infoLogFile)
		} else {
			logWriter = infoLogFile
		}
	} else {
		// If no log file, use stdout for terminal output or discard if silenced
		if logTerminal && !silence {
			logWriter = os.Stdout
		} else {
			logWriter = io.Discard
		}
	}

	if bucketLogger.Logger == nil {
		bucketLogger.Logger = log.New(logWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return bucketLogger
}

func WithLogger(logger *log.Logger) LogFunc {
	return func(l domain.BucketLogger) {
		l.AddLogger(logger)
	}
}

func (l *BucketLogger) AddLogger(logger *log.Logger) {
	l.Logger = logger
}

func (l *BucketLogger) GetLogger() *log.Logger {
	return l.Logger
}

// Writer returns the writer for the info logger
func (l *BucketLogger) Writer() io.Writer {
	return l.Logger.Writer()
}

// Info logs an info message
func (l *BucketLogger) Info(message string) {
	if l.silence {
		return
	}
	l.Logger.Println(message)
}

// infof logs an info message with formatting
func (l *BucketLogger) Infof(format string, args ...any) {
	if l.silence {
		return
	}
	l.Logger.Printf(format, args...)
}

// Warn logs a warning message
func (l *BucketLogger) Warn(message string) {
	if l.silence {
		return
	}
	l.Logger.Println("WARN:", message)
}

// Error logs an error message
func (l *BucketLogger) Errorf(format string, args ...any) {
	if l.silence {
		return
	}
	l.Logger.Printf("ERROR: "+format, args...)
}

// WrapError returns err wrapped with message as context using %w, so
// errors.Is / errors.As continue to work through it. It does NOT log: bucket
// returns errors to the caller rather than logging them (logging is the
// application's decision). Use Info/Warn/Errorf for diagnostics you are
// deliberately not returning. Returns nil when err is nil.
func (l *BucketLogger) WrapError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorf is like WrapError but formats message with args first.
func (l *BucketLogger) WrapErrorf(message string, err error, args ...any) error {
	if err == nil {
		return nil
	}
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	return fmt.Errorf("%s: %w", message, err)
}
