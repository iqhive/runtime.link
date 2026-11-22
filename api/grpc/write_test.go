package grpc_test

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/iqhive/runtime.link/api/grpc"
)

// parkedWriter waits inside Write, as the write end of an [io.Pipe] waits for
// its reader, and says where p is before and after the wait.
type parkedWriter struct {
	before chan uintptr
	resume chan struct{}
	after  chan uintptr
}

func (w parkedWriter) Write(p []byte) (int, error) {
	w.before <- uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	<-w.resume
	w.after <- uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	return len(p), nil
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

// A Writer may show p to another goroutine until Write returns, which is what
// an io.Pipe does: its reader copies from p while the writer waits. Encode
// once built each tag in an array on its own stack and told the compiler that
// the Writer kept nothing of it, so p pointed into the stack of the goroutine
// that called Encode, and the reader was left reading freed memory when the
// collector moved that stack. The same went for anything the caller of Encode
// had on its stack.
func TestEncodeHandsTheWriterMemoryThatStaysWhereItIs(t *testing.T) {
	type message struct {
		Number int64
		Float  float64
		Text   string
		Data   []byte
		Named  map[string]int32
		List   []uint16
	}
	var (
		w        = parkedWriter{before: make(chan uintptr), resume: make(chan struct{}), after: make(chan uintptr)}
		finished = make(chan error, 1)
	)
	go func() {
		growStack(512)
		var local [64]byte
		for i := range local {
			local[i] = byte(i)
		}
		finished <- grpc.NewEncoder(w).Encode(message{
			Number: 300,
			Float:  1.5,
			Text:   string(local[:8]),
			Data:   local[:],
			Named:  map[string]int32{string(local[8:12]): 1},
			List:   []uint16{1, 2},
		})
	}()
	writes := 0
	for {
		var before uintptr
		select {
		case err := <-finished:
			if err != nil {
				t.Fatal(err)
			}
			if writes == 0 {
				t.Fatal("Encode wrote nothing")
			}
			return
		case before = <-w.before:
		case <-time.After(30 * time.Second):
			t.Fatal("Encode neither wrote nor returned")
		}
		writes++
		runtime.GC() // the encoding goroutine is waiting inside Write
		w.resume <- struct{}{}
		if after := <-w.after; after != before {
			t.Errorf("write %d: p was at %#x, and at %#x once Write had waited: it pointed into a stack", writes, before, after)
		}
	}
}
