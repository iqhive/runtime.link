package txt

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Scanner[Syntax any] struct {
	value string
	Value Syntax
}

func (s Scanner[M]) String() string { return s.value }
func (Scanner[M]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	s := ptr.(*Scanner[M])
	n = strings.Index(raw, string(tag))
	if n >= 0 {
		raw = raw[:n]
	} else {
		n = len(raw)
	}
	var pattern Pattern[M]
	if _, err := pattern.MatchString(&pattern, raw, tag); err != nil {
		return 0, err
	}
	s.Value = *pattern.Format()
	s.value = raw
	return n + 1, nil
}

type Pattern[Syntax any] struct {
	formatMethods[Syntax]
}

type formatMethods[Syntax any] struct {
	raw string
	txt *match[Syntax]
}

func (f formatMethods[Syntax]) String() string {
	if f.txt == nil {
		if f.raw == "" {
			return ""
		}
		return "<invalid>"
	}
	return f.raw
}

func (f *formatMethods[Syntax]) Format() *Syntax {
	return f.txt.ptr
}

func (f *formatMethods[Syntax]) Err() error {
	if f.txt == nil {
		return nil
	}
	return f.txt.err
}

func (f *formatMethods[Syntax]) getFormatMethods() *formatMethods[Syntax] {
	return f
}

func (f *formatMethods[Syntax]) Get() (string, bool) {
	return f.raw, f.txt != nil && f.txt.err == nil
}

func (formatMethods[Syntax]) MatchString(ptr any, raw string, tag reflect.StructTag) (int, error) {
	txt := decode[Syntax](raw)
	err := txt.err
	if err != nil {
		return 0, err
	}
	pattern := ptr.(interface{ getFormatMethods() *formatMethods[Syntax] }).getFormatMethods()
	pattern.raw = raw
	pattern.txt = txt
	return 0, nil
}

type isPattern[Syntax any] interface {
	~struct {
		formatMethods[Syntax]
	}
}

func New[F isPattern[Syntax], Syntax any](raw string) F {
	return F{formatMethods[Syntax]{
		raw: raw,
		txt: decode[Syntax](raw),
	}}
}

type match[Syntax any] struct {
	err error
	ptr *Syntax
}

func decode[Syntax any](raw string) *match[Syntax] {
	txt := new(match[Syntax])
	txt.ptr = new(Syntax)
	var (
		rtype = reflect.TypeOf(txt.ptr).Elem()
		value = reflect.ValueOf(txt.ptr).Elem()
	)
	if rtype.Kind() != reflect.Struct {
		txt.err = fmt.Errorf("invalid txt.Pattern syntax: not a struct")
		return txt
	}
	stype := func(rtype reflect.Type) string {
		name := rtype.PkgPath() + "." + rtype.Name()
		name, _, _ = strings.Cut(name, "[")
		return name
	}
	for i := 0; i < value.NumField(); i++ {
		var (
			field = value.Field(i)
		)
		matcher, ok := field.Interface().(Matcher)
		if !ok {
			txt.err = fmt.Errorf("invalid txt.Pattern syntax: field %v (%v) does not implement txt.Matcher", rtype.Field(i).Name, stype(rtype.Field(i).Type))
			return txt
		}
		n, err := matcher.MatchString(field.Addr().Interface(), raw, rtype.Field(i).Tag)
		if err != nil {
			txt.err = fmt.Errorf("invalid txt.Pattern syntax: field %v: %w", rtype.Field(i).Name, err)
			return txt
		}
		if n >= len(raw) {
			break
		}
		raw = raw[n:]
	}
	return txt
}

// Matcher is used to match a string against a set of rules.
type Matcher interface {
	fmt.Stringer

	// MatchString advances the matcher by N bytes, returning an error if the
	// given string does not match the rules of the [Matcher].
	MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error)
}

type (
	Divide[T any] struct {
		value string

		Values []Pattern[T]
	}

	First[T Matcher] struct {
		Value T
	}
	Final[T Matcher] struct {
		Value T
	}

	Min bool
	Len bool
	Max bool

	Suffix bool
	Prefix bool

	ASCII string
)

func (a ASCII) String() string { return string(a) }
func (ASCII) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	a := ptr.(*ASCII)
	for _, char := range raw {
		if char > 127 {
			return 0, fmt.Errorf("invalid ASCII character")
		}
	}
	*a = ASCII(raw)
	return 0, nil
}

func (s Suffix) String() string { return strconv.FormatBool(bool(s)) }
func (s Suffix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasSuffix(raw, string(tag)) {
		return len(tag), nil
	}
	return 0, fmt.Errorf("missing suffix %q", string(tag))
}

func (s Prefix) String() string { return strconv.FormatBool(bool(s)) }
func (s Prefix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasPrefix(raw, string(tag)) {
		return len(tag), nil
	}
	return 0, fmt.Errorf("missing prefix %q", string(tag))
}

func (d Divide[T]) String() string { return d.value }

func (Divide[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	repeat := ptr.(*Divide[T])
	splits := strings.Split(raw, string(tag))
	repeat.Values = make([]Pattern[T], len(splits))
	for i, matcher := range repeat.Values {
		_, err := matcher.MatchString(&repeat.Values[i], splits[i], tag)
		if err != nil {
			return 0, err
		}
	}
	repeat.value = raw
	return n, nil
}

func (First[M]) first() {}
func (Final[M]) final() {}

type isFirst interface{ first() }
type isFinal interface{ final() }

func (f First[M]) String() string { return f.Value.String() }
func (First[M]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	first := ptr.(*First[M])
	return first.Value.MatchString(&first.Value, raw, tag)
}

func (f Final[M]) String() string { return f.Value.String() }
func (Final[M]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	first := ptr.(*Final[M])
	return first.Value.MatchString(&first.Value, raw, tag)
}

func (min Min) String() string { return strconv.FormatBool(bool(min)) }
func (min Min) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	i, err := strconv.Atoi(string(tag))
	if err != nil {
		return 0, fmt.Errorf("invalid txt.Min tag value: %w", err)
	}
	if len(raw) < int(i) {
		return 0, fmt.Errorf("too short")
	}
	return 0, nil
}

func (max Max) String() string { return strconv.FormatBool(bool(max)) }
func (max Max) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	i, err := strconv.Atoi(string(tag))
	if err != nil {
		return 0, fmt.Errorf("invalid txt.Min tag value: %w", err)
	}
	if len(raw) > int(i) {
		return 0, fmt.Errorf("too long")
	}
	return 0, nil
}

type WithBacktick[T Matcher] struct {
	WithBacktick T
}

func (b WithBacktick[T]) String() string { return b.WithBacktick.String() }
func (WithBacktick[T]) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	val := *(ptr.(*WithBacktick[T]))
	return val.WithBacktick.MatchString(&val.WithBacktick, raw, tag+"`")
}
