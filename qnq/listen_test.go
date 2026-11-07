package qnq_test

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/iqhive/runtime.link/qnq"
)

// decoder is one message as a [qnq.Channels] backend delivers it.
type decoder = func(any) (func(error), error)

// backend holds its channel open unless a test closes it, as a real one would
// for as long as the broker is up.
type backend struct {
	messages chan decoder
}

func newBackend() *backend { return &backend{messages: make(chan decoder, 8)} }

func (b *backend) Send(context.Context, qnq.Topic, any) error { return nil }

func (b *backend) Recv(context.Context, qnq.Topic, string) <-chan decoder { return b.messages }

// text delivers a string, reporting how the listener went through acked.
func text(value string, acked chan<- error) decoder {
	return func(into any) (func(error), error) {
		*into.(*string) = value
		return func(err error) { acked <- err }, nil
	}
}

func TestDecoderWithoutAnAckDoesNotCrash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBackend()
		acked := make(chan error, 1)
		b.messages <- func(any) (func(error), error) { return nil, errors.New("bad payload") }
		b.messages <- text("after", acked)
		close(b.messages)

		var got []string
		ch := qnq.OpenChan[string](b, "t")
		ch.Listen(t.Context(), "s", func(_ context.Context, value string) error {
			got = append(got, value)
			return nil
		})
		synctest.Wait()
		if !slices.Equal(got, []string{"after"}) {
			t.Fatalf("got %q, want the message after the bad one", got)
		}
	})
}

func TestMessageWithoutAnAckStillReachesTheListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBackend()
		b.messages <- func(into any) (func(error), error) {
			*into.(*string) = "hello"
			return nil, nil
		}
		close(b.messages)

		var got []string
		ch := qnq.OpenChan[string](b, "t")
		ch.Listen(t.Context(), "s", func(_ context.Context, value string) error {
			got = append(got, value)
			return nil
		})
		synctest.Wait()
		if !slices.Equal(got, []string{"hello"}) {
			t.Fatalf("got %q", got)
		}
	})
}

func TestDecodeFailureIsAcknowledgedAsAFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBackend()
		failure := errors.New("bad payload")
		acked := make(chan error, 1)
		b.messages <- func(any) (func(error), error) {
			return func(err error) { acked <- err }, failure
		}
		close(b.messages)

		ch := qnq.OpenChan[string](b, "t")
		ch.Listen(t.Context(), "s", func(context.Context, string) error {
			t.Error("a message that failed to decode reached the listener")
			return nil
		})
		synctest.Wait()
		if err := <-acked; !errors.Is(err, failure) {
			t.Fatalf("acked with %v", err)
		}
	})
}

// A message left unacknowledged is the backend's to redeliver, so a listener
// that has gone away must not be handed one, or acknowledge one.
func TestListenerIsNotCalledAfterItsContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBackend()
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		ch := qnq.OpenChan[string](b, "t")
		ch.Listen(ctx, "s", func(context.Context, string) error {
			calls.Add(1)
			return nil
		})
		synctest.Wait()

		cancel()
		acked := make(chan error, 1)
		b.messages <- text("late", acked)
		synctest.Wait()

		if n := calls.Load(); n != 0 {
			t.Errorf("listener called %d times after its context ended", n)
		}
		if len(acked) != 0 {
			t.Error("a message nobody processed was acknowledged")
		}
	})
}

// synctest fails this test if the goroutine Listen started never exits, which
// is what happens when a backend keeps its channel open past the context.
func TestListenGoroutineEndsWithItsContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBackend()
		ctx, cancel := context.WithCancel(t.Context())
		ch := qnq.OpenChan[string](b, "t")
		ch.Listen(ctx, "s", func(context.Context, string) error { return nil })
		synctest.Wait()
		cancel()
		synctest.Wait()
	})
}

func TestSendReachesEveryListenerWhenOneFails(t *testing.T) {
	ctx := context.Background()
	first, second := errors.New("first"), errors.New("second")
	var calls [4]atomic.Int32
	var topic qnq.Chan[string]
	for i, failure := range []error{nil, first, second, nil} {
		topic.Listen(ctx, "s", func(context.Context, string) error {
			calls[i].Add(1)
			return failure
		})
	}
	err := topic.Send(ctx, "hello")
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("Send returned %v, want both failures", err)
	}
	for i := range calls {
		if n := calls[i].Load(); n != 1 {
			t.Errorf("listener %d called %d times", i, n)
		}
	}
}

// Failing is not the same as nobody listening, a caller retries one of them.
func TestSendWhoseOnlyListenerFailsIsNotAnEmptyChannel(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("failed")
	var topic qnq.Chan[string]
	topic.Listen(ctx, "s", func(context.Context, string) error { return failure })
	err := topic.Send(ctx, "hello")
	if !errors.Is(err, failure) || errors.Is(err, qnq.ErrEmptyChannel) {
		t.Fatalf("Send returned %v", err)
	}
}

func TestCanceledListenersAreCleanedUpWhenAnotherFails(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("failed")
	canceled, cancel := context.WithCancel(ctx)
	var late atomic.Int32
	var topic qnq.Chan[string]
	topic.Listen(ctx, "s", func(context.Context, string) error { return failure })
	topic.Listen(canceled, "s", func(context.Context, string) error {
		t.Error("a canceled listener was called")
		return nil
	})
	topic.Listen(ctx, "s", func(context.Context, string) error {
		late.Add(1)
		return nil
	})
	cancel()
	if err := topic.Send(ctx, "hello"); !errors.Is(err, failure) {
		t.Fatalf("Send returned %v", err)
	}
	if late.Load() != 1 {
		t.Fatal("the listener after a canceled one missed the message")
	}
}

// Registration can take time; messages sent after Listen returns still need
// to find the subscription, without waiting for a goroutine to register it.
type registeringBackend struct {
	*backend
	release    <-chan struct{}
	registered atomic.Bool
}

func (b *registeringBackend) Recv(ctx context.Context, topic qnq.Topic, subscription string) <-chan decoder {
	<-b.release
	b.registered.Store(true)
	return b.backend.Recv(ctx, topic, subscription)
}

func (b *registeringBackend) Send(ctx context.Context, topic qnq.Topic, value any) error {
	if !b.registered.Load() {
		return qnq.ErrEmptyChannel
	}
	b.messages <- func(into any) (func(error), error) { *into.(*string) = value.(string); return nil, nil }
	return nil
}

func TestListenRegistersBeforeReturning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		release := make(chan struct{})
		released := false
		defer func() {
			if !released {
				close(release)
			}
		}()
		b := &registeringBackend{backend: newBackend(), release: release}
		ch := qnq.OpenChan[string](b, "topic")
		result := make(chan error, 1)
		var got []string
		go func() {
			ch.Listen(ctx, "subscription", func(_ context.Context, value string) error { got = append(got, value); return nil })
			result <- ch.Send(ctx, "immediate")
		}()
		synctest.Wait()
		select {
		case err := <-result:
			t.Fatalf("Listen returned before registration completed; immediate Send returned %v", err)
		default:
		}
		close(release)
		released = true
		synctest.Wait()
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, []string{"immediate"}) {
			t.Fatalf("got %q", got)
		}
	})
}
