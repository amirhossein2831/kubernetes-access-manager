package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
	return &lumberjack.Logger{Filename: c.Output.Path, MaxSize: int(c.Output.MaxBytes / (1024 * 1024)), MaxBackups: c.Output.Backups, Compress: false, LocalTime: false}, nil
}
