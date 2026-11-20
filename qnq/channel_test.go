package qnq

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
)

func TestConcurrentCancellationRemovesIdleSubscriptions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var topic Chan[int]
		var registrations sync.WaitGroup
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		for range 32 {
			registrations.Go(func() {
				topic.Listen(ctx, "temporary", func(context.Context, int) error {
					t.Error("called a canceled listener")
					return nil
				})
			})
		}
		calls := 0
		topic.Listen(t.Context(), "active", func(context.Context, int) error {
			calls++
			return nil
		})
		registrations.Wait()
		cancel()
		synctest.Wait()

		if got := len(*topic.fast.listeners.Load()); got != 1 {
			t.Fatalf("queue retained %d subscriptions after cancellation, want 1", got)
		}
		if err := topic.Send(t.Context(), 1); err != nil || calls != 1 {
			t.Fatalf("Send returned %v and delivered %d messages, want one delivery", err, calls)
		}
	})
}
