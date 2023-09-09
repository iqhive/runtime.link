package ffi

type Handle[T any] struct {
}

type File Handle[File]

type JumpBuffer Handle[JumpBuffer]
type (
	FilePosition Handle[FilePosition]
)

type Buffer struct{}
