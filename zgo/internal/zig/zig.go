package zig

import (
	"context"

	"runtime.link/api"
)

type Command struct {
	api.Specification

	Init  func(context.Context) error                  `args:"init"`
	Build func(context.Context) error                  `args:"build"`
	Run   func(ctx context.Context, file string) error `args:"run %v"`
}
