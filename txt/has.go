package txt

import (
	"reflect"
	"strings"
)

type (
	Has[This isThen, Else isElse] struct {
		tru bool
		Has This
		Not Else
	}
	Then[T any] Pattern[T]
	Else[T any] Pattern[T]
)

type isThen interface {
	Matcher
	isThen()
}

func (Then[T]) isThen() {}

func (s Then[T]) String() string {
	return Pattern[T](s).String()
}

func (Then[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := (*Pattern[T])(ptr.(*Then[T]))
	return pfx.MatchString(pfx, raw, tag)
}

type isElse interface {
	Matcher
	isElse()
}

func (Else[T]) isElse() {}

func (Else[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := (*Pattern[T])(ptr.(*Else[T]))
	return pfx.MatchString(pfx, raw, tag)
}

func (pfx Has[This, Else]) String() string {
	if pfx.tru {
		return pfx.Has.String()
	}
	return pfx.Not.String()
}
func (Has[This, Else]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	pfx := ptr.(*Has[This, Else])
	if strings.HasPrefix(raw, string(tag)) {
		pfx.tru = true
		return pfx.Has.MatchString(&pfx.Has, raw, tag)
	}
	return pfx.Not.MatchString(&pfx.Not, raw, tag)
}
