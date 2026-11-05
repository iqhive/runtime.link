package example

import (
	"context"
	"embed"
	"io/fs"

	"github.com/iqhive/runtime.link/api"
)

//go:embed *.go
var source embed.FS

// Celsius is a defined type, which has to arrive as itself.
type Celsius float32

// Point is a struct with no references in it, so it crosses by value.
type Point struct {
	X, Y int32
	Name string
}

type API struct {
	api.Specification

	HelloWorld func()

	HostArch func() string

	Add func(a, b int) int

	Echo func(text string) (echoed string)

	Scalars func(a int8, b uint16, c bool, d float32, e complex64, f uintptr) string

	Warm func(t Celsius) Celsius

	Move func(ctx context.Context, from Point, by [2]int32) (Point, error)

	Join func(sep string, parts []string) string

	Sum func(values ...int) int

	Check func(text string) error

	Split func(text string) (head, tail string)

	Conjugate func(c complex128) complex128

	Keys func(m map[string]int) int

	Mirror func(p *Point)

	// Spread has as many wasm params as Go params, but they don't line up.
	Spread func(nothing struct{}, c complex128)

	// Partly names one parameter of two.
	Partly func(known int, _ int)
}

func (API) Source() fs.FS { return source }
