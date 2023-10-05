// Package xray provides standard means for introspecting the internal operational state of an [api.Linker].
package xray

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"sync"
)

type key struct{}

type values struct {
	mutex sync.RWMutex
	value map[reflect.Type]chan any
}

// New returns a new xray context, that can have values added to a queue for
// future introspection.
func New(ctx context.Context) context.Context {
	return context.WithValue(ctx, key{}, &values{
		value: make(map[reflect.Type]chan any),
	})
}

// Off returns a new xray context that will not record any values.
func Off(ctx context.Context) context.Context {
	return context.WithValue(ctx, key{}, nil)
}

// Add a value to the xray context, it can be retrieved later with Get.
func Add(ctx context.Context, value any) {
	var set, ok = ctx.Value(key{}).(*values)
	if !ok {
		return
	}
	set.mutex.RLock()
	defer set.mutex.RUnlock()
	ch, ok := set.value[reflect.TypeOf(value)]
	if !ok {
		ch = make(chan any, 1)
		set.value[reflect.TypeOf(value)] = ch
	}
	if ch == nil {
		ch = make(chan any, 1)
	}
	if len(ch) == cap(ch) {
		grow := make(chan any, cap(ch)*2)
		for elem := range ch {
			grow <- elem
		}
		ch = grow
	}
	set.value[reflect.TypeOf(value)] = ch
	ch <- value
}

// Has returns true if the xray context has a value of the given type.
func Has[T any](ctx context.Context) bool {
	set, ok := ctx.Value(key{}).(*values)
	if !ok {
		return false
	}
	set.mutex.RLock()
	defer set.mutex.RUnlock()
	ch, ok := set.value[reflect.TypeOf([0]T{}).Elem()]
	return ok && len(ch) > 0
}

// Get a value from the xray context, if it exists. Otherwise a zero
// value is returned.
func Get[T any](ctx context.Context) T {
	var zero T
	var set = ctx.Value(key{}).(*values)
	set.mutex.RLock()
	defer set.mutex.RUnlock()
	var ch, ok = set.value[reflect.TypeOf([0]T{}).Elem()]
	if !ok {
		return zero
	}
	if len(ch) > 0 {
		select {
		case v := <-ch:
			return v.(T)
		case <-ctx.Done():
			return zero
		}
	}
	return zero
}

// Reader with additional methods for introspecting what has
// been read.
type Reader interface {
	io.Reader
	io.Closer

	String() string
	Bytes() []byte
}

// NewReader will wrap the provided [io.Reader] to ensure that it
// implements [Reader] if an xray is enabled.
func NewReader(ctx context.Context, r io.Reader) Reader {
	if r, ok := r.(Reader); ok {
		return r
	}
	closer, ok := r.(io.ReadCloser)
	if !ok {
		closer = io.NopCloser(r)
	}
	_, on := ctx.Value(key{}).(*values)
	if !on {
		return offReader{closer}
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return offReader{errReader{err}}
	}
	return reader{
		Buffer: bytes.NewBuffer(b),
		Reader: bytes.NewReader(b),
	}
}

type reader struct {
	*bytes.Buffer
	*bytes.Reader
}

func (r reader) Read(p []byte) (int, error) { return r.Reader.Read(p) }
func (r reader) Close() error               { return nil }

type offReader struct {
	io.ReadCloser
}

func (r offReader) Bytes() []byte  { return []byte(r.String()) }
func (r offReader) String() string { return "xray not enabled" }

type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }
func (r errReader) Close() error               { return nil }

// Writer with additional methods for introspecting what has been
// written.
type Writer interface {
	io.Writer
	io.Closer

	String() string
	Bytes() []byte
}

// NewWriter will wrap the provided [io.Writer] to ensure that it
// implements [Writer] if an xray is enabled.
func NewWriter(ctx context.Context, w io.Writer) Writer {
	if w, ok := w.(Writer); ok {
		return w
	}
	closer, ok := w.(io.WriteCloser)
	if !ok {
		closer = nopCloser{w}
	}
	_, on := ctx.Value(key{}).(*values)
	if !on {
		return offWriter{closer}
	}
	return writer{
		Buffer:      bytes.NewBuffer(nil),
		WriteCloser: closer,
	}
}

type offWriter struct {
	io.WriteCloser
}

func (w offWriter) Bytes() []byte  { return []byte(w.String()) }
func (w offWriter) String() string { return "xray not enabled" }

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

type writer struct {
	*bytes.Buffer
	io.WriteCloser
}

func (w writer) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if err != nil {
		return n, err
	}
	return w.WriteCloser.Write(p)
}

func (w writer) Bytes() []byte  { return w.Buffer.Bytes() }
func (w writer) String() string { return w.Buffer.String() }
