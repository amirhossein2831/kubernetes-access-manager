package logger

import (
	"errors"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/output"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type core struct {
	writer *output.Writer
	level  zapcore.Level
	fields []zapcore.Field
}

func New(w *output.Writer, level string) (*zap.Logger, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	return zap.New(&core{writer: w, level: l}), nil
}
func (c *core) Enabled(l zapcore.Level) bool { return l >= c.level }
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
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message", LevelKey: "level", TimeKey: "timestamp", EncodeLevel: zapcore.LowercaseLevelEncoder, EncodeTime: zapcore.ISO8601TimeEncoder})
	enc.AddString("schema_version", "v1")
	enc.AddString("event_type", "operational")
	enc.AddString("event_id", output.NewID())
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
func (c *core) Sync() error { return nil }
