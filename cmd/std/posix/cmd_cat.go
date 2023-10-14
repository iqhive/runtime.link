package posix

import (
	"context"

	"runtime.link/api"
)

// CatCommand type.
type CatCommand StandardCatCommand[CatOptions]

// StandardCatCommand line program.
type StandardCatCommand[Options interface{ cat() CatOptions }] struct {
	api.Documentation `exec:"cat"
		concatenates files together and returns them.`

	Files func(ctx context.Context, paths Paths, opts *Options) error `args:"%[2]v %[1]v"
		prints the given files.`
}

// CatOptions for [CatCommand].
type CatOptions struct {
	Common

	Buffer bool `args:"-u,invert"`
}

func (args CatOptions) cat() CatOptions { return args }
