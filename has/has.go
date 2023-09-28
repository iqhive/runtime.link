package has

import (
	"reflect"
	"strings"

	"runtime.link/txt"
)

type (
	Prefix[This isThis, Else isElse] struct {
		tru bool
		Has This
		Not Else
	}
	This[T any] txt.Pattern[T]
	Else[T any] txt.Pattern[T]
)

type isThis interface {
	txt.Matcher
	isThis()
}

func (This[T]) isThis() {}

func (s This[T]) String() string {
	return txt.Pattern[T](s).String()
}

func (This[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := (*txt.Pattern[T])(ptr.(*This[T]))
	return pfx.MatchString(pfx, raw, tag)
}

type isElse interface {
	txt.Matcher
	isElse()
}

func (Else[T]) isElse() {}

func (Else[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := (*txt.Pattern[T])(ptr.(*Else[T]))
	return pfx.MatchString(pfx, raw, tag)
}

func (pfx Prefix[This, Else]) String() string {
	if pfx.tru {
		return pfx.Has.String()
	}
	return pfx.Not.String()
}
func (Prefix[This, Else]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := ptr.(*Prefix[This, Else])
	if strings.HasPrefix(raw, string(tag)) {
		pfx.tru = true
		return pfx.Has.MatchString(&pfx.Has, raw, tag)
	}
	return pfx.Not.MatchString(&pfx.Not, raw, tag)
}
