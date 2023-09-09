package ffi

import "runtime.link/box"

type File box.As[File]

type JumpBuffer box.As[JumpBuffer]
type (
	FilePosition box.As[FilePosition]
)

type Buffer struct{}
