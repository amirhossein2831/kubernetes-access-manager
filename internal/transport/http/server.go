package http

import (
	"context"
	"errors"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net"
	"net/http"
	"time"
)

func NewServer(lc fx.Lifecycle, c *config.Config, h http.Handler, m *observability.Metrics, log *zap.Logger, shutdown fx.Shutdowner) *http.Server {
	s := &http.Server{Addr: c.HTTP.Addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	lc.Append(fx.Hook{OnStart: func(context.Context) error {
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			return err
		}
		go func() {
			if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				m.SetReady(false)
				log.Error("HTTP server failed")
				_ = shutdown.Shutdown(fx.ExitCode(1))
			}
		}()
		return nil
	}, OnStop: func(ctx context.Context) error { m.SetReady(false); return s.Shutdown(ctx) }})
	return s
}
