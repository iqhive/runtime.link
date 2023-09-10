package posix

import (
	"context"
	"io"

	"runtime.link/cmd"
)

// CatCommand type.
type CatCommand StandardCatCommand[CatOptions]

// StandardCatCommand line program.
type StandardCatCommand[Options interface{ cat() CatOptions }] struct {
	cmd.Line `cmd:"cat"
		concatenates files together and returns them.`

	Files func(ctx context.Context, paths Paths, opts *Options) error `cmd:"%[2]v %[1]v"
		prints the given files.`
}

// CatArguments for [CatCommand].
type CatOptions struct {
	Common

	Buffer bool `cmd:"-u,invert"`

	Reader io.Reader `cmd:",omitempty"`
	Writer io.Writer `cmd:",omitempty"`
}

func (args CatOptions) cat() CatOptions { return args }
