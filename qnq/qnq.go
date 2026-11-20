/*
Package qnq provides a representation for message queuing, event handling and
callback flows. Where there is a fan-out messaging pattern, ie. Pub/Sub semantics.

Variables of type [Chan] are references to a message queue, T-typed messages can be
sent to the [Chan] using the [Chan.Send] method and/or subscriptions can be
attached by calling [Chan.Listen] with a [Listener].

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

	NameChanges.Listen(ctx, "customers", func(ctx context.Context, event NameChangeEvent) error {
	   // update our copy of the customer
	   return nil
	})
*/
package qnq

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync/atomic"
	"weak"

	"github.com/iqhive/runtime.link/api/xray"
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

	// Recv a channel of messages from the named channel and subscription ID
	// each message is a decoder that returns an acknowledgement function
	// which should be called as soon as processing
	// is complete. The function may be nil if the message
	// cannot be acknowledged, such as when it fails to decode.
	// The subscription must be ready for new sends when Recv returns.
	Recv(ctx context.Context, topic Topic, subscription string) <-chan func(any) (func(error), error)
}

// Chan is similar to a Go channel, except that each value sent to it is
// broadcast to each registered [Listener]. All operations on a [Chan] are
// goroutine safe and lockless. At-least-once delivery semantics.
type Chan[T any] struct {
	fast channel[T]
	impl Channels
	name Topic
}

// Send will broadcast the given message to all registered listeners, with at-least-once
// delivery. If a reciever returns an error, the other listeners still receive the
// message, the errors will be returned by [Chan.Send] and it is the caller's
// responsibility to retry the send operation, which delivers to every listener again.
// [Chan.Send] will return an error if no listeners are registered. For an
// in-process channel, listeners added during dispatch start with the next send.
func (ch *Chan[T]) Send(ctx context.Context, value T) error {
	if ch.impl != nil {
		return ch.impl.Send(ctx, ch.name, value)
	}
	return ch.fast.send(ctx, value)
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
func (ch *Chan[T]) Listen(ctx context.Context, subscription string, listener Listener[T]) {
	if ch.impl != nil {
		// Establish the subscription before returning: a send immediately
		// after Listen must already have this listener as a recipient.
		messages := ch.impl.Recv(ctx, ch.name, subscription)
		go ch.receive(ctx, messages, listener)
	} else {
		ch.fast.register(ctx, listener)
	}
}

// receive runs until the context ends, whether or not the backend closes its
// channel. A message taken after that is left unacknowledged, for the backend
// to redeliver, as nobody is left to process it.
func (ch *Chan[T]) receive(ctx context.Context, messages <-chan func(any) (func(error), error), listener Listener[T]) {
	for {
		select {
		case <-ctx.Done():
			return
		case decode, open := <-messages:
			if !open || ctx.Err() != nil {
				return
			}
			deliver(ctx, decode, listener)
		}
	}
}

func deliver[T any](ctx context.Context, decode func(any) (func(error), error), listener Listener[T]) {
	var message T
	ack, err := decode(&message)
	if err == nil {
		if ctx.Err() != nil {
			return
		}
		err = listener(ctx, message)
	}
	if ack != nil {
		ack(err)
	}
}

type Topic string

func (ch *Chan[T]) open(mq Channels, name Topic) {
	ch.impl = mq
	ch.name = name
}

// Open a Channels structure, with each field being a [Chan] with a 'qnq' tag
// that specifies the name that will be used to call [OpenChan] on it.
func Open[T any](db Channels) *T {
	type opener interface {
		open(db Channels, name Topic)
	}
	var zero T
	rtype := reflect.TypeOf(zero)
	value := reflect.ValueOf(&zero).Elem()
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if topic, ok := field.Tag.Lookup("qnq"); ok {
			if field.Type.Kind() == reflect.Pointer {
				value.Field(i).Set(reflect.New(field.Type.Elem()))
				value.Field(i).Interface().(opener).open(db, Topic(topic))
			} else {
				value.Field(i).Addr().Interface().(opener).open(db, Topic(topic))
			}
		}
	}
	return &zero
}

// OpenChan opens a single [Chan] with the given [Channels] implementation and topic name.
// This is useful when different channels in a struct need different backends.
func OpenChan[T any](db Channels, name Topic) *Chan[T] {
	var ch Chan[T]
	ch.open(db, name)
	return &ch
}

// Listener for values of type Message on a [Chan].
type Listener[Message any] func(context.Context, Message) error

type subscription[T any] struct {
	ctx    context.Context
	handle Listener[T]
}

type channel[T any] struct {
	// Published slices are immutable. Each send uses the registrations it
	// observed at the start, even when a handler registers another listener.
	listeners atomic.Pointer[[]*subscription[T]]
}

func (ch *channel[T]) register(ctx context.Context, listener Listener[T]) {
	if ctx.Err() != nil {
		return
	}
	sub := &subscription[T]{ctx, listener}
	for {
		old := ch.listeners.Load()
		var next []*subscription[T]
		if old != nil {
			next = make([]*subscription[T], len(*old), len(*old)+1)
			copy(next, *old)
		}
		next = append(next, sub)
		if ch.listeners.CompareAndSwap(old, &next) {
			break
		}
	}
	if ctx.Done() != nil {
		// A long-lived context must allow an unused channel and its
		// listeners to be collected before cancellation.
		queue, entry := weak.Make(ch), weak.Make(sub)
		context.AfterFunc(ctx, func() {
			if ch := queue.Value(); ch != nil {
				ch.remove(entry.Value())
			}
		})
	}
}

func (ch *channel[T]) remove(sub *subscription[T]) {
	for {
		old := ch.listeners.Load()
		i := slices.Index(*old, sub)
		if i < 0 {
			return
		}
		next := make([]*subscription[T], 0, len(*old)-1)
		next = append(next, (*old)[:i]...)
		next = append(next, (*old)[i+1:]...)
		if ch.listeners.CompareAndSwap(old, &next) {
			return
		}
	}
}

func (ch *channel[T]) send(ctx context.Context, message T) error {
	listeners := ch.listeners.Load()
	if listeners == nil {
		return ErrEmptyChannel
	}
	sent := false
	var failures []error
	for _, sub := range *listeners {
		if sub.ctx.Err() != nil {
			continue
		}
		if err := sub.handle(ctx, message); err != nil {
			failures = append(failures, err)
		}
		sent = true
	}
	if !sent {
		return ErrEmptyChannel
	}
	return xray.New(errors.Join(failures...))
}
