package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"solo_quest_backend/internal/config"
)

var L *zap.Logger

type rotatingFileSyncer struct {
	mu          sync.Mutex
	dir         string
	baseName    string
	ext         string
	maxBytes    int64
	maxBackups  int
	currentFile *os.File
	currentSize int64
}

func newRotatingFileSyncer(dir, fileName string, maxSizeMB, maxBackups int) (*rotatingFileSyncer, error) {
	ext := filepath.Ext(fileName)
	baseName := strings.TrimSuffix(fileName, ext)
	if ext == "" {
		ext = ".log"
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	s := &rotatingFileSyncer{
		dir:        dir,
		baseName:   baseName,
		ext:        ext,
		maxBytes:   int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
	}

	if err := s.openFile(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *rotatingFileSyncer) openFile() error {
	path := filepath.Join(s.dir, s.baseName+s.ext)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}

	s.currentFile = f
	s.currentSize = info.Size()
	return nil
}

func (s *rotatingFileSyncer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentSize+int64(len(p)) >= s.maxBytes {
		if err := s.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = s.currentFile.Write(p)
	s.currentSize += int64(n)
	return n, err
}

func (s *rotatingFileSyncer) rotate() error {
	if s.currentFile != nil {
		s.currentFile.Close()
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	oldPath := filepath.Join(s.dir, s.baseName+s.ext)
	newPath := filepath.Join(s.dir, fmt.Sprintf("%s-%s%s", s.baseName, timestamp, s.ext))

	if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename log file: %w", err)
	}

	s.cleanupOldBackups()

	return s.openFile()
}

func (s *rotatingFileSyncer) cleanupOldBackups() {
	pattern := filepath.Join(s.dir, s.baseName+"-*"+s.ext)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= s.maxBackups {
		return
	}

	oldest := len(matches) - s.maxBackups
	for i := 0; i < oldest; i++ {
		os.Remove(matches[i])
	}
}

func (s *rotatingFileSyncer) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentFile != nil {
		return s.currentFile.Sync()
	}
	return nil
}

func (s *rotatingFileSyncer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentFile != nil {
		return s.currentFile.Close()
	}
	return nil
}

type InitOption func(*config.LoggingConfig)

func WithLogFolder(folder string) InitOption {
	return func(c *config.LoggingConfig) {
		c.FileFolder = folder
	}
}

func WithLogFileName(name string) InitOption {
	return func(c *config.LoggingConfig) {
		c.FileName = name
	}
}

func Init(cfg *config.Config, opts ...InitOption) {
	lc := cfg.Logging
	for _, opt := range opts {
		opt(&lc)
	}

	level := detectLogLevel(lc.Level)
	encoder := buildEncoder(cfg.AppEnv, lc)

	core := buildLoggerCore(cfg, lc, level, encoder)

	L = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	L = L.With(
		zap.String("env", cfg.AppEnv),
		zap.String("service_name", cfg.ServiceName),
	)

	SetSlowOperationThreshold(lc.SlowOperationMs)
}

func detectLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func buildEncoder(appEnv string, lc config.LoggingConfig) zapcore.Encoder {
	cfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if lc.WriteToFiles {
		return zapcore.NewConsoleEncoder(cfg)
	}

	switch strings.ToLower(lc.Format) {
	case "console", "text":
		return zapcore.NewConsoleEncoder(cfg)
	default:
		return zapcore.NewJSONEncoder(cfg)
	}
}

func buildLoggerCore(cfg *config.Config, lc config.LoggingConfig, level zapcore.Level, encoder zapcore.Encoder) zapcore.Core {
	var cores []zapcore.Core

	if lc.WriteToFiles {
		logDir := filepath.Join(lc.FileDir, lc.FileFolder)
		syncer, err := newRotatingFileSyncer(logDir, lc.FileName, lc.FileMaxSizeMB, lc.FileMaxBackups)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create rotating file syncer: %v\n", err)
		} else {
			fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				TimeKey:        "timestamp",
				LevelKey:       "level",
				NameKey:        "logger",
				CallerKey:      "caller",
				MessageKey:     "message",
				StacktraceKey:  "stacktrace",
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.LowercaseLevelEncoder,
				EncodeTime:     zapcore.ISO8601TimeEncoder,
				EncodeDuration: zapcore.MillisDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			})
			cores = append(cores, zapcore.NewCore(fileEncoder, syncer, level))
		}

		consoleCfg := zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
		consoleEncoder := zapcore.NewConsoleEncoder(consoleCfg)
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level))
	} else {
		var writeSyncer zapcore.WriteSyncer
		outputs := strings.Split(lc.Output, ",")
		for _, output := range outputs {
			output = strings.TrimSpace(output)
			switch output {
			case "stderr":
				writeSyncer = zapcore.AddSync(os.Stderr)
			default:
				writeSyncer = zapcore.AddSync(os.Stdout)
			}
		}
		cores = append(cores, zapcore.NewCore(encoder, writeSyncer, level))
	}

	return zapcore.NewTee(cores...)
}

func Sync() {
	if L != nil {
		L.Sync()
	}
}

func InitForTest() {
	if L != nil {
		return
	}
	L = zap.NewNop()
}
