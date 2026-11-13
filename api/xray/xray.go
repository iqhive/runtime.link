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

// NewContext returns a new xray context, that can have values added to a queue for
// future introspection.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, key{}, &values{
		value: make(map[reflect.Type]chan any),
	})
}

// ContextOff returns a new xray context that will not record any values.
func ContextOff(ctx context.Context) context.Context {
	return context.WithValue(ctx, key{}, nil)
}

// ContextAdd a value to the xray context, it can be retrieved later with Get.
func ContextAdd(ctx context.Context, value any) {
	var set, ok = ctx.Value(key{}).(*values)
	if !ok {
		return
	}
	set.mutex.Lock()
	defer set.mutex.Unlock()
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
		close(ch)
		for elem := range ch {
			grow <- elem
		}
		ch = grow
	}
	set.value[reflect.TypeOf(value)] = ch
	ch <- value
}

// ContextHas returns true if the xray context has a value of the given type.
func ContextHas[T any](ctx context.Context) bool {
	set, ok := ctx.Value(key{}).(*values)
	if !ok {
		return false
	}
	set.mutex.RLock()
	defer set.mutex.RUnlock()
	ch, ok := set.value[reflect.TypeOf([0]T{}).Elem()]
	return ok && len(ch) > 0
}

// ContextGet a value from the xray context, if it exists. Otherwise a zero
// value is returned.
func ContextGet[T any](ctx context.Context) T {
	var zero T
	var set, on = ctx.Value(key{}).(*values)
	if !on {
		return zero
	}
	set.mutex.RLock()
	defer set.mutex.RUnlock()
	var ch, ok = set.value[reflect.TypeOf([0]T{}).Elem()]
	if !ok {
		return zero
	}
	// Readers run concurrently under the read lock, so a value seen in the
	// channel can be gone by the time it is received. Waiting for it would
	// hold the lock against the only writer that could supply another.
	select {
	case v := <-ch:
		return v.(T)
	default:
		return zero
	}
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
		return offReader{errReader{err: err, source: closer}}
	}
	return reader{
		Reader: bytes.NewReader(b),
		record: b,
		source: closer,
	}
}

// reader keeps what was read in a plain slice. A [bytes.Buffer] embedded here
// would lend its Write, ReadFrom and ReadString, which act on the record and
// not on what is being read.
type reader struct {
	*bytes.Reader

	record []byte
	source io.Closer
}

func (r reader) Close() error   { return r.source.Close() }
func (r reader) Bytes() []byte  { return r.record }
func (r reader) String() string { return string(r.record) }

type offReader struct {
	io.ReadCloser
}

func (r offReader) Bytes() []byte  { return []byte(r.String()) }
func (r offReader) String() string { return "xray not enabled" }

type errReader struct {
	err    error
	source io.Closer
}

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }
func (r errReader) Close() error               { return r.source.Close() }

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
		record: bytes.NewBuffer(nil),
		target: closer,
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

// writer names its fields instead of embedding them: an embedded
// [bytes.Buffer] lends its WriteString and ReadFrom, which [io.WriteString]
// and [io.Copy] prefer to Write, and those fill the record without writing
// anything to the target.
type writer struct {
	record *bytes.Buffer
	target io.WriteCloser
}

// Write records what the target took, which is less than p when the write
// failed or was cut short.
func (w writer) Write(p []byte) (int, error) {
	n, err := w.target.Write(p)
	w.record.Write(p[:max(0, min(n, len(p)))])
	return n, err
}

func (w writer) Close() error   { return w.target.Close() }
func (w writer) Bytes() []byte  { return w.record.Bytes() }
func (w writer) String() string { return w.record.String() }
