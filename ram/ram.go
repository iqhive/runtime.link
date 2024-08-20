package ram

import (
	"context"
)

type Reader interface {
	Read(ctx context.Context, p []byte) (n Bytes, err error)
}

type Writer interface {
	Write(ctx context.Context, p []byte) (n Bytes, err error)
}

// Bytes count.
type Bytes uintptr

type Global[V any] interface {
	Get(context.Context) (V, error)
	Set(context.Context, V) error
}
