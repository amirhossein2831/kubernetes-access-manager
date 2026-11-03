package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/client/kubernetes"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/infrastructure"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/jobs"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/output"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service/monitor"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

func options(c *config.Config) fx.Option {
	return fx.Options(
		fx.Supply(c),
		fx.Provide(func() ([]domain.User, error) { return config.LoadInventory(c.InputPath) }, func() (domain.Policy, error) { return config.LoadPolicy(c.PolicyPath) }, prometheus.NewRegistry, observability.New, infrastructure.NewRotator,
			func(lc fx.Lifecycle, file *lumberjack.Logger, m *observability.Metrics) *output.Writer {
				w := output.New(file, os.Stdout)
				w.Observe = m.Output
				w.Emergency = os.Stderr
				lc.Append(fx.Hook{OnStop: func(context.Context) error { return file.Close() }})
				return w
			},
			func(w *output.Writer) (*zap.Logger, error) { return logger.New(w, c.Log.Level) },
			func(m *observability.Metrics) (*kubernetes.Client, error) {
				client, err := kubernetes.New(c)
				if err == nil {
					client.Observe = m.Request
				}
				return client, err
			},
			func(users []domain.User, p domain.Policy, client *kubernetes.Client, w *output.Writer, m *observability.Metrics, log *zap.Logger) *monitor.Service {
				return monitor.New(c, users, p, client, w, m, log)
			},
		), transport.Module,
		fx.Invoke(func(lc fx.Lifecycle, s *monitor.Service, m *observability.Metrics) {
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			started := false
			lc.Append(fx.Hook{OnStart: func(context.Context) error {
				started = true
				m.SetReady(true)
				go func() { defer close(done); jobs.Run(ctx, c.Poll.Interval, s.Collect) }()
				return nil
			}, OnStop: func(stop context.Context) error {
				m.SetReady(false)
				cancel()
				if !started {
					return nil
				}
				select {
				case <-done:
					return nil
				case <-stop.Done():
					return stop.Err()
				}
			}})
		}),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log.WithOptions(zap.IncreaseLevel(zap.WarnLevel))}
		}),
		fx.StartTimeout(30*time.Second), fx.StopTimeout(c.HTTP.ShutdownTimeout),
	)
}
func main() {
	dir := os.Getenv("APP_CONFIG_DIR")
	if dir == "" {
		dir = "configs"
	}
	c, err := config.LoadFrom(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "access-monitor: invalid startup configuration:", err)
		os.Exit(1)
	}
	fx.New(options(c)).Run()
}
