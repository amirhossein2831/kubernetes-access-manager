package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestImmediateNonoverlappingAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var active, calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, 10*time.Millisecond, func(ctx context.Context) error {
			if active.Add(1) != 1 {
				t.Error("overlapping runs")
			}
			defer active.Add(-1)
			if calls.Add(1) == 2 {
				cancel()
				return nil
			}
			select {
			case <-ctx.Done():
			case <-time.After(35 * time.Millisecond):
			}
			return nil
		})
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop failed to stop")
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}
