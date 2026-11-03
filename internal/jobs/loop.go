package jobs

import (
	"context"
	"time"
)

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
