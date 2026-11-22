package jrpc_test

import (
	"io"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/iqhive/runtime.link/api/jrpc"
)

// parked says where p is before and after a wait inside Read or Write, which
// is where a Read of a connection waits for data and the write end of an
// [io.Pipe] waits for its reader.
type parked struct {
	text   string
	before chan uintptr
	resume chan struct{}
	after  chan uintptr
}

func newParked(text string) *parked {
	return &parked{text: text, before: make(chan uintptr), resume: make(chan struct{}), after: make(chan uintptr)}
}

func (p *parked) wait(buf []byte) {
	p.before <- uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	<-p.resume
	p.after <- uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
}

func (p *parked) Read(buf []byte) (int, error) {
	p.wait(buf)
	if p.text == "" {
		return 0, io.EOF
	}
	// A few bytes at a time, so that a document takes several reads.
	n := copy(buf[:min(len(buf), 4)], p.text)
	p.text = p.text[n:]
	return n, nil
}

func (p *parked) Write(buf []byte) (int, error) {
	p.wait(buf)
	return len(buf), nil
}

// moved lets the goroutine that is waiting in p carry on once the collector
// has run, and reports whether what it was given was somewhere else by then.
// finished ends the wait if the goroutine returned without reading or writing.
func (p *parked) moved(t *testing.T, finished <-chan error) (isWaiting, hasMoved bool) {
	t.Helper()
	var before uintptr
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
		return false, false
	case before = <-p.before:
	case <-time.After(30 * time.Second):
		t.Fatal("neither read, wrote nor returned")
	}
	runtime.GC()
	p.resume <- struct{}{}
	return true, <-p.after != before
}

// growStack leaves the goroutine with a stack far larger than it goes on to
// use, which the collector shrinks, moving it, while the goroutine waits.
//
//go:noinline
func growStack(depth int) int {
	var frame [1024]byte
	if depth == 0 {
		return int(frame[0])
	}
	return growStack(depth-1) + int(frame[depth%len(frame)])
}

// The buffer a Decoder reads into is inside the Decoder, which NewDecoder
// returns by value. The compiler was told that a Reader kept nothing of it, so
// a Decoder in a local variable stayed on the stack of its goroutine, and so
// did the p its Reader was given. Whatever filled p from outside that
// goroutine while Read waited, another goroutine or the kernel finishing an
// overlapped read, wrote into freed memory once the collector had moved the
// stack.
func TestDecoderHandsTheReaderMemoryThatStaysWhereItIs(t *testing.T) {
	var (
		r        = newParked(`{"n": 42} [1, 2]`)
		finished = make(chan error, 1)
		values   = make(chan [2]any, 1)
	)
	go func() {
		growStack(512)
		dec := jrpc.NewDecoder(r)
		var (
			object map[string]int
			list   []int
		)
		if err := dec.Decode(&object); err != nil {
			finished <- err
			return
		}
		err := dec.Decode(&list)
		values <- [2]any{object["n"], len(list)}
		finished <- err
	}()
	reads := 0
	for {
		isWaiting, hasMoved := r.moved(t, finished)
		if !isWaiting {
			break
		}
		reads++
		if hasMoved {
			t.Errorf("read %d: p was somewhere else once Read had waited: it pointed into a stack", reads)
		}
	}
	if reads == 0 {
		t.Fatal("Decode never read")
	}
	if got := <-values; got != [2]any{42, 2} {
		t.Errorf("decoded %v", got)
	}
}

// An Encoder builds its message in a buffer of its own before the Writer sees
// any of it, so what the Writer is given stays where it is too.
func TestEncoderHandsTheWriterMemoryThatStaysWhereItIs(t *testing.T) {
	var (
		w        = newParked("")
		finished = make(chan error, 1)
	)
	go func() {
		growStack(512)
		var local [16]byte
		copy(local[:], "on the stack")
		finished <- jrpc.NewEncoder(w).Encode(map[string]any{"text": string(local[:12]), "n": 1})
	}()
	writes := 0
	for {
		isWaiting, hasMoved := w.moved(t, finished)
		if !isWaiting {
			break
		}
		writes++
		if hasMoved {
			t.Errorf("write %d: p was somewhere else once Write had waited: it pointed into a stack", writes)
		}
	}
	if writes != 1 {
		t.Errorf("%d writes, want the message in one", writes)
	}
}
