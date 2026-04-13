package core

import (
	"noyo/core/config"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// GlobalLogBroadcaster broadcasts log messages to active WebSocket clients
	GlobalLogBroadcaster = NewLogBroadcaster()
)

type LogMessage struct {
	Time    string                 `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Logger  string                 `json:"logger"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

type Broadcaster struct {
	mu        sync.Mutex
	listeners map[chan LogMessage]bool
}

func NewLogBroadcaster() *Broadcaster {
	return &Broadcaster{
		listeners: make(map[chan LogMessage]bool),
	}
}

func (b *Broadcaster) AddListener(ch chan LogMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[ch] = true
}

func (b *Broadcaster) RemoveListener(ch chan LogMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.listeners, ch)
	close(ch)
}

func (b *Broadcaster) Broadcast(msg LogMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.listeners {
		select {
		case ch <- msg:
		default:
			// drop if buffer full
		}
	}
}

// wsCore is a custom zapcore that broadcasts logs
type wsCore struct {
	zapcore.LevelEnabler
	enc zapcore.Encoder
}

func (c *wsCore) With(fields []zapcore.Field) zapcore.Core {
	return &wsCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc.Clone(),
	}
}

func (c *wsCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *wsCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	msg := LogMessage{
		Time:    ent.Time.Format("2006-01-02 15:04:05.000"),
		Level:   ent.Level.String(),
		Message: ent.Message,
		Logger:  ent.LoggerName,
		Fields:  enc.Fields,
	}
	GlobalLogBroadcaster.Broadcast(msg)
	return nil
}

func (c *wsCore) Sync() error {
	return nil
}

// InitLogger creates a new zap.Logger based on the configuration
func InitLogger(cfg config.LogConfig) *zap.Logger {
	level := zap.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Console encoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// File encoder
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	if cfg.Dir == "" {
		cfg.Dir = "./data/logs"
	}
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		panic(err)
	}

	lumberjackLogger := &lumberjack.Logger{
		Filename:   cfg.Dir + "/noyo.log",
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxDays,
		Compress:   cfg.Compress,
	}

	// Check if the existing log file is from a previous day and rotate it
	if info, err := os.Stat(cfg.Dir + "/noyo.log"); err == nil {
		if info.ModTime().Format("2006-01-02") != time.Now().Format("2006-01-02") {
			lumberjackLogger.Rotate()
		}
	}

	// Start a background goroutine to rotate logs daily at midnight
	go func(l *lumberjack.Logger) {
		for {
			now := time.Now()
			// Calculate time until next midnight
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			duration := next.Sub(now)
			time.Sleep(duration)
			l.Rotate()
		}
	}(lumberjackLogger)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
		zapcore.NewCore(fileEncoder, zapcore.AddSync(lumberjackLogger), level),
		&wsCore{LevelEnabler: level, enc: fileEncoder},
	)

	return zap.New(core, zap.AddCaller())
}
