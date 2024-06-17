package zig

import (
	"context"

	"runtime.link/api"
)

type Command struct {
	api.Specification

	Init  func(context.Context)                  `args:"init"`
	Build func(context.Context)                  `args:"build"`
	Run   func(ctx context.Context, file string) `args:"run %v"`
	Test  func(ctx context.Context, file string) `args:"test %v"`
}
