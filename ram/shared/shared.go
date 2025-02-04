package shared

import (
	"io"
	"iter"
	"time"
)

type MemoryAddress string
type MemoryLayout string

type Function string

type Memory interface {
	// Update opens a memory address for writing, such that when the memory writer is closed, the resulting address
	// will either store the written bytes or will have been updated more recently by another update. If the memory
	// layout does not match one of the provided layouts, an error is returned.
	Update(layouts ...MemoryLayout) (io.WriteCloser, MemoryLayout, error)

	// Append opens a memory address for appending, such that when the memory writer is closed, such that as long as
	// the memory address has not been updated, rewritten, or removed, the resulting address will contain the written
	// bytes. If the memory layout does not match the provided layout, an error is returned.
	Append(layouts ...MemoryLayout) (io.WriteCloser, MemoryLayout, error)

	// Latest opens an exclusive read/write on the memory address, such that the resulting address will contain the
	// most recent bytes written to the address and will be blocked from other operations until the read/write is closed.
	//
	// If no write is made to the address, the data stored in the memory address will remain unchanged. If the memory
	// layout does not match one of the provided layouts, an error is returned.
	Latest(layouts ...MemoryLayout) (io.ReadWriteCloser, int, MemoryLayout, error)

	// Cached opens a read-only view of the memory address, such that the resulting address will contain a result with
	// the most recent bytes written to the address, up to the maximum age provided. If the resulting memory layout does
	// not match one of the provided layouts, an error is returned.
	Cached(maxage time.Duration, layouts ...MemoryLayout) (io.ReadCloser, int, MemoryLayout, error)

	// Result returns an input-stream for calling the specified function on the given memory address. If the memory
	// layout does not match one of the provided layouts, an error is returned.
	Result(fn Function, layouts ...MemoryLayout) (io.WriteCloser, MemoryLayout, error)

	// Remove removes all data from the given memory address, such that the address will no longer contain any data.
	// Returns an error if called within sliced memory.
	Remove() error

	// Mapped returns mapped memory, such that the resulting memory can be treated as if it were in an isolated addressing
	// namespace. Updates made inside the mapped memory will eventually be notified as a write here.
	Mapped(MemoryAddress) Memory

	// Sliced returns a slice of the memory address, such that the resulting memory will contain only the bytes between
	// the given start and end indexes. If the memory layout does not match one of the provided layouts, an error is returned.
	Sliced(idx int64, end int64, layouts ...MemoryLayout) (Memory, MemoryLayout, error)

	// Notify returns a channel that will eventually be notified whenever a write affects [Memory].
	Notify(layouts ...MemoryLayout) <-chan MemoryWrite

	// Search returns a sequence of mapped memory addresses that match the given prefix and are currently stored as one of the
	// provided layouts.
	Search(prefix MemoryAddress, layouts ...MemoryLayout) iter.Seq2[MemoryAddress, MemoryLayout]
}

type MemoryWriter interface {
	io.WriteCloser
	SetLayout(layout MemoryLayout)
}

type MemoryReader interface {
	io.ReadCloser
	Layout() MemoryLayout
}

type MemoryReadWriter interface {
	MemoryWriter
	MemoryReader
}

type MemoryWrite struct {
	Mapped MemoryAddress
	Layout MemoryLayout
	Offset int64
	Buffer []byte
}
