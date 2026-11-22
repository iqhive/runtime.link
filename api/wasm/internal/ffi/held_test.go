package ffi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api/wasm/internal/ffi"
)

// The tests of what a guest holds work under a bound of 1MiB with strings of
// a quarter of that, so that the fifth string a guest keeps is one too many
// and one that keeps a string or two is far from it, and need no memory to
// speak of.
const (
	heldCap   = 1 << 20
	heldChunk = 256 << 10
)

// refuses runs what a guest does and reports whether the bridge refused it
// with a panic, which is how a bridge function traps its guest.
func refuses(does func()) (refused bool) {
	defer func() { refused = recover() != nil }()
	does()
	return false
}

// fewerHeld lowers the bound on what a guest holds for the length of a test,
// and returns a new bridge and the text of one chunk.
func fewerHeld(t *testing.T) (ffi.API, string) {
	t.Helper()
	handles, held := ffi.FewerCaps(1024, heldCap)
	t.Cleanup(func() { ffi.FewerCaps(handles, held) })
	return ffi.New(), strings.Repeat("a", heldChunk)
}

// The bound on what the handles of a guest hold of the host counted a string
// on the handle that string_new returned, and stopped counting it when that
// handle was freed. A guest that had first put the string somewhere else it
// holds kept the bytes and got the count back: 32 strings of 512KiB under a
// bound of 1MiB, of which the bridge counted a few hundred bytes. The count
// follows the memory now, wherever the guest has put it.
func TestHeldBytesFollowWhatAGuestMovesOutOfItsHandle(t *testing.T) {
	const times = 32

	t.Run("into a variable that pointer_new made", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		// The type of a string, the way a guest gets it: struct_type of a
		// string that it holds.
		seed := bridge.String.New(strings.NewReader("x"), 1)
		stringType := bridge.Structure.Type(ffi.Structure(seed))
		bridge.String.Free(seed)

		var kept []ffi.Pointer
		refused := refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				pointer := bridge.Pointer.New(stringType)
				bridge.Structure.Encode.String(bridge.Pointer.Data(pointer), text)
				bridge.String.Free(text)
				kept = append(kept, pointer)
			}
		})
		if !refused || len(kept)*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of %d bytes in pointers (refused: %v)", heldCap, len(kept), heldChunk, refused)
		}
		// What it kept is still there to read.
		for _, pointer := range kept {
			text := bridge.Structure.Decode.String(bridge.Pointer.Data(pointer))
			if got := bridge.String.Len(text); got != heldChunk {
				t.Errorf("a string that was kept reads as %d bytes", got)
			}
			bridge.String.Free(text)
		}
		// Freed, the pointers make room for as many again.
		for _, pointer := range kept {
			bridge.Pointer.Free(pointer)
		}
		if refuses(func() { bridge.String.Free(bridge.String.New(strings.NewReader(chunk), heldChunk)) }) {
			t.Error("a guest that freed what it kept was still refused")
		}
	})

	t.Run("into the elements of a slice", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		function := bridge.NewFunction(func([]string) {})
		slice := bridge.Slice.Make(bridge.Type.Arg(bridge.Structure.Type(ffi.Structure(function)), 0), times, times)
		kept := 0
		refused := refuses(func() {
			for i := range uint32(times) {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				// The handle of the element is freed as well as the one of
				// the string: the slice holds the string then.
				element := bridge.Slice.Elem(slice, i)
				bridge.Structure.Encode.String(bridge.Pointer.Data(element), text)
				bridge.Pointer.Free(element)
				bridge.String.Free(text)
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of %d bytes in a slice (refused: %v)", heldCap, kept, heldChunk, refused)
		}
	})

	t.Run("into the arguments of a call that is never made", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		function := bridge.NewFunction(func(string) {})
		kept := 0
		refused := refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				bridge.Structure.Encode.String(bridge.Function.Args(function), text)
				bridge.String.Free(text)
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of %d bytes in arguments (refused: %v)", heldCap, kept, heldChunk, refused)
		}
	})

	t.Run("into a channel", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		function := bridge.NewFunction(func(chan string) {})
		channel := bridge.Channel.Make(bridge.Type.Arg(bridge.Structure.Type(ffi.Structure(function)), 0), times)
		kept := 0
		refused := refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				bridge.Structure.Encode.String(bridge.Channel.Data(channel), text)
				bridge.String.Free(text)
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of %d bytes in a channel (refused: %v)", heldCap, kept, heldChunk, refused)
		}
		// Received and freed, they are held no longer.
		for range kept {
			bridge.String.Free(bridge.Structure.Decode.String(bridge.Channel.Data(channel)))
		}
		if refuses(func() { bridge.String.Free(bridge.String.New(strings.NewReader(chunk), heldChunk)) }) {
			t.Error("a guest that received and freed what it sent was still refused")
		}
	})

	t.Run("into what a decoder reads from", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		kept := 0
		refused := refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				bridge.String.Data(text)
				bridge.String.Free(text)
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of %d bytes behind decoders (refused: %v)", heldCap, kept, heldChunk, refused)
		}
	})
}

// A slice among the arguments of a call grows by an element for every call
// the guest makes, on one handle, for as long as the guest goes on. Each
// element is counted with what the guest holds.
func TestHeldBytesCountTheSliceAnArgumentGrowsTo(t *testing.T) {
	bridge, _ := fewerHeld(t)
	var passed int
	function := bridge.NewFunction(func(numbers []int64) { passed = len(numbers) })
	arguments := bridge.Function.Args(function)
	const elements = 4 * heldCap / 8
	sent := 0
	refused := refuses(func() {
		bridge.Structure.Encode.Int64(arguments, elements)
		for i := range int64(elements) {
			bridge.Structure.Encode.Int64(arguments, i)
			sent++
		}
	})
	if !refused || sent*8 > heldCap {
		t.Errorf("under a bound of %d bytes the arguments of one call grew to a slice of %d bytes (refused: %v)", heldCap, sent*8, refused)
	}

	// A slice that fits is passed as it always was.
	bridge, _ = fewerHeld(t)
	function = bridge.NewFunction(func(numbers []int64) { passed = len(numbers) })
	arguments = bridge.Function.Args(function)
	bridge.Structure.Encode.Int64(arguments, 1024)
	for i := range int64(1024) {
		bridge.Structure.Encode.Int64(arguments, i)
	}
	bridge.Function.Call(context.Background(), function, arguments)
	if passed != 1024 {
		t.Errorf("a slice of 1024 numbers arrived as %d", passed)
	}
}

// What the host gives a guest is the host's memory too: the results of a call
// for as long as the guest keeps the handle it reads them from, and a string
// that an exported function returns.
func TestHeldBytesCountWhatTheHostGives(t *testing.T) {
	t.Run("the results of a call", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		function := bridge.NewFunction(func() string { return strings.Clone(chunk) })
		kept := 0
		refused := refuses(func() {
			for range 32 {
				bridge.Function.Call(context.Background(), function, bridge.Function.Args(function))
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept the results of %d calls, %d bytes each (refused: %v)", heldCap, kept, heldChunk, refused)
		}
	})
	t.Run("a string of the host's", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		kept := 0
		refused := refuses(func() {
			for range 32 {
				bridge.NewString(strings.Clone(chunk))
				kept++
			}
		})
		if !refused || kept*heldChunk > heldCap {
			t.Errorf("under a bound of %d bytes a guest kept %d strings of the host's, %d bytes each (refused: %v)", heldCap, kept, heldChunk, refused)
		}
	})
}

// A handle to an element of a slice holds the whole array of the slice, even
// once the slice is freed. The one that Slice.Elem gives was counted for it,
// and the one that a guest gets by reading a slice as something it is not
// was not.
func TestHeldBytesCountTheArrayBehindAnElement(t *testing.T) {
	bridge, _ := fewerHeld(t)
	function := bridge.NewFunction(func([]int64) {})
	numbers := bridge.Type.Arg(bridge.Structure.Type(ffi.Structure(function)), 0)
	kept := 0
	refused := refuses(func() {
		for range 32 {
			slice := bridge.Slice.Make(numbers, heldChunk/8, heldChunk/8)
			bridge.Structure.Decode.Chan(bridge.Slice.Data(slice))
			bridge.Slice.Free(slice)
			kept++
		}
	})
	if !refused || kept*heldChunk > heldCap {
		t.Errorf("under a bound of %d bytes a guest kept %d arrays of %d bytes behind handles to an element (refused: %v)", heldCap, kept, heldChunk, refused)
	}
}

// A guest that frees what it is given, or puts each string where the last one
// was, never holds more than a string or two, however many pass through it:
// 64 strings of 256KiB are 16 times the bound.
func TestHeldBytesAreHandedBackWhenNothingHoldsThem(t *testing.T) {
	const times = 64

	t.Run("arguments and results of calls", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		var passed int
		function := bridge.NewFunction(func(text string) string {
			passed += len(text)
			return text[:16]
		})
		if refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				arguments := bridge.Function.Args(function)
				bridge.Structure.Encode.String(arguments, text)
				bridge.String.Free(text)
				results := bridge.Function.Call(context.Background(), function, arguments)
				bridge.Structure.Free(arguments)
				result := bridge.Structure.Decode.String(results)
				bridge.String.Free(result)
				bridge.Structure.Free(results)
			}
		}) {
			t.Errorf("a guest that frees everything it is given was refused after %d bytes of arguments", passed)
		}
		if passed != times*heldChunk {
			t.Errorf("%d bytes of arguments arrived, want %d", passed, times*heldChunk)
		}
	})

	t.Run("one variable written over and over", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		seed := bridge.String.New(strings.NewReader("x"), 1)
		variable := bridge.Pointer.New(bridge.Structure.Type(ffi.Structure(seed)))
		bridge.String.Free(seed)
		written := 0
		if refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				bridge.Structure.Encode.String(bridge.Pointer.Data(variable), text)
				bridge.String.Free(text)
				written++
			}
		}) {
			t.Errorf("a guest that keeps one string at a time was refused at the %dth", written+1)
		}
	})

	t.Run("strings that are read and freed", func(t *testing.T) {
		bridge, chunk := fewerHeld(t)
		read := 0
		if refuses(func() {
			for range times {
				text := bridge.String.New(strings.NewReader(chunk), heldChunk)
				data := bridge.String.Data(text)
				bridge.Structure.Decode.Uint8(data)
				bridge.Structure.Free(data)
				bridge.String.Free(text)
				read++
			}
		}) {
			t.Errorf("a guest that frees every string it reads was refused at the %dth", read+1)
		}
	})
}
