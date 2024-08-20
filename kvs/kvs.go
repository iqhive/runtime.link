package kvs

import (
	"context"
)

type Location interface {
	Load(context.Context, string, any) error
	Save(context.Context, string, any) error
	List(context.Context, string) (chan string, chan error)
}

type Map[K comparable, V any] struct {
	at Location
	as string
}
