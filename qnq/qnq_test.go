package qnq_test

import (
	"context"
	"sync/atomic"
	"testing"

	"runtime.link/qnq"
)

func TestPubSub(t *testing.T) {
	ctx := context.Background()

	var handled atomic.Int32

	var topic = qnq.Chan[string]{}
	topic.Listen(ctx, "1", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	cctx, cancel := context.WithCancel(ctx)
	topic.Listen(cctx, "2", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	topic.Listen(ctx, "3", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})
	topic.Listen(ctx, "4", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	if err := topic.Send(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if handled.Load() != 4 {
		t.Fatal("expected 4 handlers to be called")
	}

	cancel()

	handled.Store(0)

	if err := topic.Send(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if handled.Load() != 3 {
		t.Fatal("expected 3 handlers to be called")
	}
}
