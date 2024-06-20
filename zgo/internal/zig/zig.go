package zig

import (
	"context"

	"runtime.link/api"
)

type Command struct {
	api.Specification

	Build func(ctx context.Context)              `args:"build"`
	Run   func(ctx context.Context, file string) `args:"run %v"`
	Test  func(ctx context.Context, file string) `args:"test %v"`
}
