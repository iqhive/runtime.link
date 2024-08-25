package ram

import (
	"context"
	"io"
)

type Reader = io.Reader

type Writer = io.Writer

type Global[V any] interface {
	Get(context.Context) (V, error)
	Set(context.Context, V) error
}
