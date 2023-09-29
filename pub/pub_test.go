package pub_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"runtime.link/pub"
)

func TestPubSub(t *testing.T) {
	ctx := context.Background()

	var handled atomic.Int32

	var topic pub.Sub[string]
	topic.Handle(ctx, func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	cctx, cancel := context.WithCancel(ctx)
	topic.Handle(cctx, func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	topic.Handle(ctx, func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})
	topic.Handle(ctx, func(ctx context.Context, s string) error {
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

func TestSplit(t *testing.T) {
	var ctx = context.Background()
	var ch = make(chan string, 10)
	var jobs, pipe = pub.Split(func(ctx context.Context, value string) error {
		ch <- value
		return nil
	})
	type key string
	var (
		goctx = context.WithValue(ctx, key("go"), "routine")
		wg    sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		for val := range ch {
			pipe.Send(goctx, val)
		}
		wg.Done()
	}()
	var (
		handlerErr error = errors.New("handler not called")
	)
	jobs.Handle(ctx, func(ctx context.Context, s string) error {
		if ctx != goctx {
			handlerErr = fmt.Errorf("expected handler to be called in goroutine")
		} else {
			handlerErr = nil
		}
		return nil
	})
	if err := jobs.Send(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	close(ch)
	wg.Wait()
	if handlerErr != nil {
		t.Fatal(handlerErr)
	}
}
