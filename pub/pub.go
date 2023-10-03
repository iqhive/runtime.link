/*
Package pub provides a representation for message queuing, event handling and
callback flows. Where there is a fan-out messaging pattern, ie. Pub/Sub semantics.

Variables of type [pub.Sub] are references to a message queue, T-typed messages can be
sent to the [pub.Sub] using the [Sub.Send] method and/or subscriptions can be
attached by calling [Sub.Handle] with a [SubHandler].

These message queues can be used to model event streams, let's say you want to model
an event that is fired whenever somebody changes their name,

	type NameChangeEvent struct {
	    Customer string
	    OldName string
	    NewName string
	}

	var NameChanges pub.Sub[NameChangeEvent]

At the location of your updateName function you would send an event:

	err := NameChanges.Send(ctx, NameChangeEvent{ ... })

Another system may want to listen to these events, so that they can
update their customer records. This system can subscribe to the name
changes queue that was passed (as a dependency) to their execution.

	NameChanges.Handle(ctx, func(ctx context.Context, event NameChangeEvent) error {
	   // update our copy of the customer
	   return nil
	})
*/
package pub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ctxBackground = context.Background()

// SubHandler for values of type Message on a [Chan].
type SubHandler[Message any] func(context.Context, Message) error

// Sub is similar to a Go channel, except that each values sent to it is
// broadcast to every registered [Handler]. Each Handler will have it's own
// queue and messages will be processed in an undefined order across multiple
// goroutines. All operations on a [Sub] are goroutine safe and lockless. A
// [Sub] value can be used immediately and does not require any initialisation
// to use. At least once delivery semantics.
type Sub[Message any] struct {
	_ [0]sync.Mutex // nocopy

	// if true, handlers are attached to 'next' and sends are only delivered
	// to this subs's handler. Only ever set on creation, immutable.
	pipe bool
	done atomic.Bool // closed?

	// if read is true, the value of context and handler
	// are permitted to be read.
	read atomic.Bool
	next atomic.Pointer[Sub[Message]] // next in the linked list.

	// unless context == context.Background(), the first handler in a linked list of
	// Chans will be nil, this is because we need to be able to delete the chan when
	// the context is cancelled and we cannot remove the head of the list as it used
	// as the entry point.
	handler SubHandler[Message]
	context context.Context
}

/*
Pipe can be used to control the delivery mechanism for a [Sub], all sends on
the returned [Sub] will *only* hit the Pipe's handler, any subsequent handlers
attached on the returned [Sub] will only be called when a message is sent to
the secondary 'handlers' [Sub].

For example (you can use Go channels as the [Sub] delivery mechanism):

	var ch = make(chan string, 10)
	var free = make(chan struct{})
	var jobs, handlers = pub.Pipe(func(ctx context.Context, value string) error {
		select {
		case <-free:
			close(ch)
			return errors.New("channel closed")
		case ch <- value:
			return nil
		}
	})
	go func() {
		for val := range ch {
			handlers.Send(val)
		}
	}()
*/
func Pipe[Message any](delivery SubHandler[Message]) (sub, handlers *Sub[Message]) {
	handlers = new(Sub[Message])
	sub = &Sub[Message]{
		pipe:    true,
		handler: delivery,
	}
	sub.next.Store(handlers)
	return
}

// Handle attaches the given handler for the lifetime of the context,
// any messages delivered to the channel after this function returns
// will trigger the given handler to be called to process this message.
// The handler must return a nil error in order to acknowledge the message
// (otherwise it will not be removed from the [Sub]). The handler should
// process incoming messages idempotently, as they may be delivered more
// than once.
func (head *Sub[Message]) Handle(ctx context.Context, handler SubHandler[Message]) {
	if head == nil {
		return
	}
	if head.pipe {
		head.next.Load().Handle(ctx, handler)
		return
	}

	// if we can attach this zero channel to a node in the list
	// then we have permission to write to the node before it.
	var zero = new(Sub[Message])

	// if the context is the background context, it will never be cancelled
	// and we can use the head of the list to store the handler, optimises
	// for the fast path where only a single global handler is registered.
	if ctx == ctxBackground {

		// if compare and swap returns true, then we can safely mutate the head
		// of the list.
		if head.next.CompareAndSwap(nil, zero) {
			head.context = ctx
			head.handler = handler
			head.read.Store(true)
			return
		}
	}

	// otherwise we need to find the end of the list and attach the new
	// handler to the end.
	node := head
	for {
		next := node.next.Load()
		if next == nil {
			if node.next.CompareAndSwap(nil, zero) {
				node.context = ctx
				node.handler = handler
				node.read.Store(true)
				return
			}
			continue
		} else {
			// delete/cleanup from the list, only safe to do if the resulting next is not nil.

			if read := next.read.Load(); read && next.context.Err() != nil {
				if nextnext := next.next.Load(); nextnext != nil {
					if node.next.CompareAndSwap(next, nextnext) {
						next = nextnext
					}
				}
			}
		}
		node = next
	}
}

// Send will broadcast the given message to all registered handlers, at least once
// delivery, so recievers must be prepared to handle duplicate messages.
func (head *Sub[Message]) Send(ctx context.Context, message Message) error {
	if head == nil {
		return errors.New("nil pub.Sub")
	}
	if head.pipe {
		return head.handler(ctx, message)
	}
	node := head
	for {
		if node == nil {
			return nil
		}
		next := node.next.Load()
		if read := node.read.Load(); read {
			if err := node.handler(ctx, message); err != nil {
				return err
			}
			if next != nil {
				// delete/cleanup from the list, only safe to do if the resulting next is not nil.
				if read = next.read.Load(); read && next.context.Err() != nil {
					if nextnext := next.next.Load(); nextnext != nil {
						if node.next.CompareAndSwap(next, nextnext) {
							next = nextnext
						}
					}
				}
			}
		}
		node = next
	}
}
