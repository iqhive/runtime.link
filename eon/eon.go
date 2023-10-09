// Package eon provides to represent pending asynchronous execution.
package eon

import (
	"context"
	"sync"
	"time"

	"runtime.link/xyz"
)

// Map a key, to the context needed for asynchronous execution for an item.
type Map[ID comparable, Item any] interface {
	// Wait asyncronously waits until the given time then executes [Map.When]
	// with the given [time.Time], ID and Item. If the [time.Time] is in the past,
	// [Map.When] will be executed immediately. If the id already exists, the
	// Item will be overwritten if the provided [time.Time] is later than both
	// the existing [Timer.Start] and [Timer.Until] times, otherwise Wait returns
	// false and the item will not be scheduled for asynchronous execution.
	Wait(context.Context, ID, time.Time, Item) (bool, error)
	// When registers a handler to be called whenever an ID and Item
	// are ready. The [time.Time] passed to the handler is the time that the
	// Item was scheduled to be processed at (not the [time.Now] when the
	// handler was called).
	When(context.Context, WaitFunc[ID, Item])
	// View returns the latest observable Item and timer information for
	// the given ID.
	View(context.Context, ID) (Item, Timer, error)
	// List returns a channel that will send all scheduled and unprocessed
	// Items between the given [time.Time]s.
	List(context.Context, time.Time, time.Time) chan xyz.Quad[ID, Item, Timer, error]
	// Stop cancels any pending executions for the given ID.
	Stop(context.Context, ID) error
}

type WaitFunc[ID comparable, Item any] func(context.Context, time.Time, ID, Item) error

// New returns a new in-memory [Map] that runs in-process and responds to
// sends on the provided channel to update the current time and process
// scheduled Items. If the channel is nil, the [Map] will run in real-time
// and use the [time] package as the basis for all scheduling operations.
// Passing a channel makes the [Map] useful for testing, where the flow of
// time can be controlled by the test. The waitgroup can be used to wait for
// any pending asynchronous operations to complete.
func New[ID comparable, Item any](now <-chan time.Time) (Map[ID, Item], *sync.WaitGroup) {
	if now == nil {
		rt := &realtime[ID, Item]{
			items: make(map[ID]Item),
			timer: make(map[ID]Timer),
			stops: make(map[ID]*time.Timer),
		}
		return rt, &rt.group
	}
	var ft = new(faketime[ID, Item])
	ft.items = make(map[ID]Item)
	ft.timer = make(map[ID]Timer)
	go func() {
		for t := range now {
			ft.cycle(t)
		}
	}()
	return ft, &ft.group
}

// Timer details for an entry in a [Map].
type Timer struct {
	Start time.Time // when the timer was created, may be later than [Until].
	Until time.Time // waiting until this time.
	Tried time.Time // last time the timer returned from calling [Map.When].
	Error error     // error from the last time the timer tried to fire.
}

type realtime[ID comparable, Item any] struct {
	mutex sync.RWMutex
	group sync.WaitGroup
	items map[ID]Item
	timer map[ID]Timer
	stops map[ID]*time.Timer
	after []func(context.Context, time.Time, ID, Item) error
}

func (rt *realtime[ID, Item]) run(ctx context.Context, t time.Time, id ID, item Item) error {
	for _, after := range rt.after {
		if err := after(ctx, t, id, item); err != nil {
			timer := rt.timer[id]
			rt.timer[id] = Timer{
				Start: timer.Start,
				Until: timer.Until,
				Tried: t,
				Error: err,
			}
			return err
		}
	}
	delete(rt.timer, id)
	delete(rt.items, id)
	delete(rt.stops, id)
	rt.group.Done()
	return nil
}

func (rt *realtime[ID, Item]) Wait(ctx context.Context, id ID, t time.Time, item Item) (bool, error) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	if timer, ok := rt.timer[id]; ok {
		if t.Before(timer.Until) || t.Before(timer.Start) {
			return false, nil
		}
	}
	if t.Before(time.Now()) {
		err := rt.run(ctx, t, id, item)
		return err == nil, err
	}
	rt.items[id] = item
	rt.timer[id] = Timer{
		Start: time.Now(),
		Until: t,
	}
	rt.stops[id] = time.NewTimer(t.Sub(time.Now()))
	rt.group.Add(1)
	ch := time.After(t.Sub(time.Now()))
	go func() {
		t, ok := <-ch
		if !ok {
			return
		}
		var backoff = time.Millisecond // TODO better heuristic
		for {
			rt.mutex.Lock()
			defer rt.mutex.Unlock()
			if err := rt.run(context.Background(), t, id, rt.items[id]); err == nil {
				return
			}
			time.Sleep(backoff)
			backoff *= 2
		}
	}()
	return true, nil
}

func (rt *realtime[ID, Item]) When(ctx context.Context, after WaitFunc[ID, Item]) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	rt.after = append(rt.after, after)
}

func (rt *realtime[ID, Item]) View(ctx context.Context, id ID) (Item, Timer, error) {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()
	return rt.items[id], rt.timer[id], nil
}

func (rt *realtime[ID, Item]) List(ctx context.Context, from, until time.Time) chan xyz.Quad[ID, Item, Timer, error] {
	var ch = make(chan xyz.Quad[ID, Item, Timer, error])
	go func() {
		rt.mutex.RLock()
		defer rt.mutex.RUnlock()
		defer close(ch)
		for id, item := range rt.items {
			timer := rt.timer[id]
			if timer.Until.Before(from) || timer.Until.After(until) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- xyz.NewQuad(id, item, timer, error(nil)):
			}
		}
	}()
	return ch
}

func (rt *realtime[ID, Item]) Stop(ctx context.Context, id ID) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	if timer, ok := rt.stops[id]; ok {
		delete(rt.timer, id)
		delete(rt.items, id)
		delete(rt.stops, id)
		timer.Stop()
		rt.group.Done()
	}
	return nil
}

type faketime[ID comparable, Item any] struct {
	mutex sync.RWMutex
	group sync.WaitGroup
	items map[ID]Item
	timer map[ID]Timer
	after []func(context.Context, time.Time, ID, Item) error
	faked time.Time
}

func (rt *faketime[ID, Item]) cycle(t time.Time) {
	for id, timer := range rt.timer {
		if t.Before(timer.Until) || timer.Until == timer.Start {
			continue
		}
		rt.run(context.Background(), t, id, rt.items[id])
	}
}

func (rt *faketime[ID, Item]) run(ctx context.Context, t time.Time, id ID, item Item) error {
	for _, after := range rt.after {
		if err := after(ctx, t, id, item); err != nil {
			timer := rt.timer[id]
			rt.timer[id] = Timer{
				Start: timer.Start,
				Until: timer.Until,
				Tried: t,
				Error: err,
			}
			return err
		}
	}
	delete(rt.timer, id)
	delete(rt.items, id)
	rt.group.Done()
	return nil
}

func (ft *faketime[ID, Item]) Wait(ctx context.Context, id ID, t time.Time, item Item) (bool, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	if timer, ok := ft.timer[id]; ok {
		if t.Before(timer.Until) || t.Before(timer.Start) {
			return false, nil
		}
	}
	if t.Before(ft.faked) {
		err := ft.run(ctx, t, id, item)
		return err == nil, err
	}
	ft.items[id] = item
	ft.timer[id] = Timer{
		Start: time.Now(),
		Until: t,
	}
	ft.group.Add(1)
	return true, nil
}

func (ft *faketime[ID, Item]) When(ctx context.Context, after WaitFunc[ID, Item]) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	ft.after = append(ft.after, after)
}

func (ft *faketime[ID, Item]) View(ctx context.Context, id ID) (Item, Timer, error) {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	return ft.items[id], ft.timer[id], nil
}

func (ft *faketime[ID, Item]) List(ctx context.Context, from, until time.Time) chan xyz.Quad[ID, Item, Timer, error] {
	var ch = make(chan xyz.Quad[ID, Item, Timer, error])
	go func() {
		ft.mutex.RLock()
		defer ft.mutex.RUnlock()
		defer close(ch)
		for id, item := range ft.items {
			timer := ft.timer[id]
			if timer.Until.Before(from) || timer.Until.After(until) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- xyz.NewQuad(id, item, timer, error(nil)):
			}
		}
	}()
	return ch
}

func (ft *faketime[ID, Item]) Stop(ctx context.Context, id ID) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	_, ok := ft.timer[id]
	if ok {
		delete(ft.timer, id)
		ft.group.Done()
	}
	return nil
}
