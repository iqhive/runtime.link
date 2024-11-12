package zig

import (
	"context"

	"runtime.link/api"
)

type Command struct {
	api.Specification

	Build func(ctx context.Context)              `cmdl:"build"`
	Run   func(ctx context.Context, file string) `cmdl:"run %v"`
	Test  func(ctx context.Context, file string) `cmdl:"test %v"`
}
