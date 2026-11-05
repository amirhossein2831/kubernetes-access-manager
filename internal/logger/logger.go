package logger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability/metric"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Module = fx.Module("logger",
	fx.Provide(NewRotator, InitWriter, Init),
)

type Results struct {
	File   error
	Stdout error
}

type Writer struct {
	mu           sync.Mutex
	file, stdout io.Writer
	Observe      func(sink string, err error, d time.Duration)
	Emergency    io.Writer
}

func NewRotator(c *config.Config) (*lumberjack.Logger, error) {
	if c.Output.MaxBytes%(1024*1024) != 0 {
		return nil, errors.New("rotation size must be a whole MiB")
	}
	if err := os.MkdirAll(filepath.Dir(c.Output.Path), 0700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.OpenFile(c.Output.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("open log output: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &lumberjack.Logger{
		Filename:   c.Output.Path,
		MaxSize:    int(c.Output.MaxBytes / (1024 * 1024)),
		MaxBackups: c.Output.Backups,
		Compress:   false,
		LocalTime:  false,
	}, nil
}

func InitWriter(lc fx.Lifecycle, file *lumberjack.Logger) *Writer {
	w := NewWriter(file, os.Stdout)
	w.Observe = func(sink string, err error, duration time.Duration) {
		result := "success"
		if err != nil {
			result = "failure"
		}
		metric.OutputTotal.WithLabelValues(sink, result).Inc()
		metric.OutputDuration.WithLabelValues(sink).Observe(duration.Seconds())
	}
	w.Emergency = os.Stderr
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error { return file.Close() },
	})
	return w
}

func NewWriter(file, stdout io.Writer) *Writer {
	return &Writer{
		file:   file,
		stdout: stdout,
	}
}

func (w *Writer) Write(record any) Results {
	b, err := json.Marshal(record)
	if err != nil {
		return Results{
			File:   err,
			Stdout: err,
		}
	}
	return w.WriteJSON(b)
}

func (w *Writer) WriteJSON(b []byte) Results {
	w.mu.Lock()
	defer w.mu.Unlock()
	line := append(append([]byte{}, b...), '\n')
	result := Results{}
	for _, sink := range []struct {
		name   string
		writer io.Writer
		target *error
	}{
		{
			name:   "file",
			writer: w.file,
			target: &result.File,
		},
		{
			name:   "stdout",
			writer: w.stdout,
			target: &result.Stdout,
		},
	} {
		start := time.Now()
		n, err := sink.writer.Write(line)
		if err == nil && n != len(line) {
			err = io.ErrShortWrite
		}
		*sink.target = err
		if w.Observe != nil {
			w.Observe(sink.name, err, time.Since(start))
		}
		if err != nil && w.Emergency != nil {
			fmt.Fprintf(w.Emergency, "access-monitor: %s output failed\n", sink.name)
		}
	}
	return result
}

func Init(w *Writer, c *config.Config) (*zap.Logger, error) {
	return New(w, c.Log.Level)
}

func FrameworkLogger(log *zap.Logger) fxevent.Logger {
	return &fxevent.ZapLogger{
		Logger: log.WithOptions(zap.IncreaseLevel(zap.WarnLevel)),
	}
}

type core struct {
	writer *Writer
	level  zapcore.Level
	fields []zapcore.Field
}

func New(w *Writer, level string) (*zap.Logger, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	return zap.New(&core{
		writer: w,
		level:  l,
	}), nil
}

func (c *core) Enabled(l zapcore.Level) bool {
	return l >= c.level
}

func (c *core) With(fields []zapcore.Field) zapcore.Core {
	next := *c
	next.fields = append(append([]zapcore.Field{}, c.fields...), fields...)
	return &next
}

func (c *core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return ce
}

func (c *core) Write(e zapcore.Entry, fields []zapcore.Field) error {
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "message",
		LevelKey:    "level",
		TimeKey:     "timestamp",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeTime:  zapcore.ISO8601TimeEncoder,
	})
	enc.AddString("schema_version", "v1")
	enc.AddString("event_type", "operational")
	enc.AddString("event_id", domain.NewID())
	enc.AddString("observed_at", e.Time.UTC().Format(time.RFC3339Nano))
	buffer, err := enc.EncodeEntry(e, append(append([]zapcore.Field{}, c.fields...), fields...))
	if err != nil {
		return err
	}
	defer buffer.Free()
	b := buffer.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	r := c.writer.WriteJSON(b)
	return errors.Join(r.File, r.Stdout)
}

func (c *core) Sync() error {
	return nil
}
