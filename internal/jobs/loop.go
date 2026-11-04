package jobs

import (
	"context"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability/health"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service/monitor"
	"go.uber.org/fx"
	"time"
)

func Init(lc fx.Lifecycle, c *config.Config, s *monitor.Service) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	started := false
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			started = true
			health.Ready.Store(true)
			go func() {
				defer close(done);
			 Run(ctx, c.Poll.Interval, s.Collect)
		    }()
			return nil
		},
		OnStop: func(stop context.Context) error {
			stop, stopCancel := context.WithTimeout(stop, c.HTTP.ShutdownTimeout)
			defer stopCancel()
			health.Ready.Store(false)
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
		},
	})
}

func Run(ctx context.Context, interval time.Duration, collect func(context.Context) error) {
	next := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}
		_ = collect(ctx)
		next = next.Add(interval)
		now := time.Now()
		if !next.After(now) {
			missed := now.Sub(next)/interval + 1
			next = next.Add(missed * interval)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
