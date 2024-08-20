/*
Package qnq provides a representation for message queuing, event handling and
callback flows. Where there is a fan-out messaging pattern, ie. Pub/Sub semantics.

Variables of type [Chan] are references to a message queue, T-typed messages can be
sent to the [Chan] using the [Chan.Send] method and/or subscriptions can be
attached by calling [Chan.Register] with a [Listener].

These message queues can be used to model event streams, let's say you want to model
an event that is fired whenever somebody changes their name,

	type NameChangeEvent struct {
	    Customer string
	    OldName string
	    NewName string
	}

	var NameChanges qnq.Chan[NameChangeEvent]

At the location of your updateName function you would send an event:

	err := NameChanges.Send(ctx, NameChangeEvent{ ... })

Another system may wish to listen to these events, so that they can
update their customer records. This system can subscribe to the name
changes queue that was passed (as a dependency) to their execution.

	NameChanges.Attach(ctx, func(ctx context.Context, event NameChangeEvent) error {
	   // update our copy of the customer
	   return nil
	})
*/
package qnq

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"

	"runtime.link/api/xray"
)

type raise string

func (err raise) Error() string { return string(err) }

// ErrEmptyChannel is raised if there are no listeners on the channel.
const ErrEmptyChannel = raise("empty channel")

// Channels interface can be used to implement a delivery mechanism for [Chan].
type Channels interface {
	// Send a message to the named channel, returning an error if the message
	// was not durably buffered by an at-least-once sender, or acknowledged.
	Send(ctx context.Context, topic Topic, val any) error

	// Recv a message from the named channel and subscription ID returns an
	// acknowledgement function which should be called before the returned
	// context is cancelled. If the returned boolean is false, the channel
	// should be considered to be closed (possibly due to the given context
	// being cancelled).
	Recv(ctx context.Context, topic Topic, subscription string, val any) (context.Context, func(error), bool)
}

// Chan is similar to a Go channel, except that each value sent to it is
// broadcast to each registered [Listener]. All operations on a [Chan] are
// goroutine safe and lockless. At-least-once delivery semantics.
type Chan[T any] struct {
	fast *channel[T]
	impl Channels
	name Topic
}

// Send will broadcast the given message to all registered listeners, with at-least-once
// delivery. If a reciever returns an error, it will be returned by [Chan.Send] and it
// is the caller's responsibility to retry the send operation. [Chan.Send] will return
// an error if no listeners are registered.
func (ch Chan[T]) Send(ctx context.Context, value T) error {
	if ch.impl != nil {
		if err := ch.impl.Send(ctx, ch.name, value); err != nil {
			return err
		}
	}
	if err := ch.fast.send(ctx, value); err != nil && (err != ErrEmptyChannel || ch.impl == nil) {
		return err
	}
	return nil
}

// Listen registers the given listener for the lifetime of the context,
// any messages delivered to the channel after this function returns
// will be delivered to the specified listener. There may also be pending
// messages buffered for this listener.
//
// The subscription name identifies the listener and may be used as a
// durable buffer to ensure that messages are not lost if the listener
// is not available.
//
// The handler must return a nil error in order to acknowledge the message
// (otherwise it will not be removed from the [Chan]). The handler should
// always process incoming messages idempotently, as they may be delivered
// more than once.
func (ch Chan[T]) Listen(ctx context.Context, subscription string, listener Listener[T]) {
	ch.fast.register(ctx, listener)
	if ch.impl != nil {
		go func() {
			var message T
			ctx, ack, ok := ch.impl.Recv(ctx, ch.name, subscription, &message)
			if !ok {
				return
			}
			ack(listener(ctx, message))
		}()
	}
}

type Topic string

func (ch *Chan[T]) open(mq Channels, name Topic) {
	ch.impl = mq
	ch.name = name
}

// Open a Channels structure, with each field being a [Chan] with a 'qnq' tag
// that specifies the name that will be used to call [OpenChan] on it.
func Open[T any](db Channels) T {
	type opener interface {
		open(db Channels, name Topic)
	}
	var zero T
	rtype := reflect.TypeOf(zero)
	value := reflect.ValueOf(&zero).Elem()
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if topic, ok := field.Tag.Lookup("qnq"); ok {
			value.Field(i).Addr().Interface().(opener).open(db, Topic(topic))
		}
	}
	return zero
}

// OpenChan opens a new [Chan] with the given name.
func OpenChan[T any](mq Channels, name Topic) Chan[T] {
	return Chan[T]{fast: new(channel[T]), impl: mq, name: name}
}

// New returns a new in-memory Chan[T].
func New[T any]() Chan[T] {
	return Chan[T]{fast: new(channel[T])}
}

// Listener for values of type Message on a [Chan].
type Listener[Message any] func(context.Context, Message) error

type channel[T any] struct {
	_ [0]sync.Mutex // nocopy
	// if true, handlers are attached to 'next' and sends are only delivered
	// to this subs's handler. Only ever set on creation, immutable.
	pipe bool
	done atomic.Bool // closed?
	// if read is true, the value of context and handler
	// are permitted to be read.
	read atomic.Bool
	next atomic.Pointer[channel[T]] // next in the linked list.
	// unless context == context.Background(), the first handler in a linked list of
	// Chans will be nil, this is because we need to be able to delete the chan when
	// the context is cancelled and we cannot remove the head of the list as it used
	// as the entry point.
	handler Listener[T]
	context context.Context
}

var ctxBackground = context.Background()

func (head *channel[T]) register(ctx context.Context, listener Listener[T]) {
	if head == nil {
		return
	}
	if head.pipe {
		head.next.Load().register(ctx, listener)
		return
	}
	// if we can attach this zero channel to a node in the list
	// then we have permission to write to the node before it.
	var zero = new(channel[T])
	// if the context is the background context, it will never be cancelled
	// and we can use the head of the list to store the handler, optimises
	// for the fast path where only a single global handler is registered.
	if ctx == ctxBackground {
		// if compare and swap returns true, then we can safely mutate the head
		// of the list.
		if head.next.CompareAndSwap(nil, zero) {
			head.context = ctx
			head.handler = listener
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
				node.handler = listener
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

func (head *channel[Message]) send(ctx context.Context, message Message) error {
	if head == nil {
		return ErrEmptyChannel
	}
	if head.pipe {
		return head.handler(ctx, message)
	}
	sent := false
	node := head
	for {
		if node == nil {
			if !sent {
				return ErrEmptyChannel
			}
			return nil
		}
		next := node.next.Load()
		if read := node.read.Load(); read {
			if err := node.handler(ctx, message); err != nil {
				return xray.New(err)
			}
			sent = true
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
