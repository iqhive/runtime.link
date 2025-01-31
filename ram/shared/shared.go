package shared

import (
	"io"
	"iter"
)

type MemoryAddress string
type MemoryLayout string

type Function string

type Memory interface {
	Create(addr MemoryAddress, layout MemoryLayout) io.WriteCloser
	Append(addr MemoryAddress, layout MemoryLayout) io.WriteCloser

	Length(addr MemoryAddress) uint
	Layout(addr MemoryAddress) MemoryLayout
	Latest(addr MemoryAddress, layout MemoryLayout) io.ReadWriteCloser
	Cached(addr MemoryAddress, layout MemoryLayout) io.ReadCloser

	Result(addr MemoryAddress, fn Function) io.WriteCloser
	Remove(addr MemoryAddress) error

	Mapped(addr MemoryAddress) Memory
	Sliced(addr MemoryAddress, layout MemoryLayout, idx int64, end int64) Memory

	Notify(addr MemoryAddress, layout MemoryLayout) chan MemoryWrite
	Search(prefix MemoryAddress, layout MemoryLayout) iter.Seq[MemoryAddress]
}

type MemoryWrite struct {
	Map    MemoryAddress
	Offset int64
	Bytes  []byte
}
