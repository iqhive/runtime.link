package ffi

import "reflect"

// reached adds up the memory of the host that the values behind handles refer
// to, each piece of it once however many of them do. It is how the bridge
// knows what a guest holds once the guest has moved it from the handle it was
// given for into something else: a string into a variable that pointer_new
// made, into an element of a slice, or into the arguments of a call.
type reached struct {
	bytes int64
	seen  map[uintptr]struct{}
}

// first reports whether the memory at address is met for the first time.
func (r *reached) first(address uintptr) bool {
	if _, met := r.seen[address]; met {
		return false
	}
	r.seen[address] = struct{}{}
	return true
}

// owns reports whether the value behind a handle is memory of its own that a
// charge of the handle counts: a string, or a slice that was made or taken of
// another. A variable holding either is addressable, and is charged for no
// more than itself.
func owns(value reflect.Value) bool {
	return !value.CanAddr() && (value.Kind() == reflect.String || value.Kind() == reflect.Slice)
}

// charged marks the memory of a handle that has a charge as met, so that it
// is not added where another value refers to it.
func (r *reached) charged(held any) {
	value, ok := held.(reflect.Value)
	if !ok || !value.IsValid() || !owns(value) {
		return
	}
	r.first(value.Pointer())
}

// handle adds what the value behind one handle refers to, without the memory
// that the charge of the handle counts. A type or a field is not a value, and
// has no memory that a guest made the host allocate.
func (r *reached) handle(held any, charged bool) {
	value, ok := held.(reflect.Value)
	if !ok || !value.IsValid() {
		return
	}
	switch state := stateOf(value).(type) {
	case *statefulDecoder:
		r.leaves(state.value)
		return
	case *statefulEncoder:
		for _, argument := range state.args {
			r.value(argument)
		}
		return
	}
	if !charged || !owns(value) {
		r.value(value)
		return
	}
	if value.Kind() == reflect.Slice {
		r.elements(value)
	}
}

// stateOf returns the decoder or encoder behind a handle, or nil if the handle
// is of anything else. What they hold is behind fields of the bridge's own,
// whether the handle is the one they were given with or one that a guest had
// made of what that points to.
func stateOf(value reflect.Value) any {
	pointer := value
	if value.Kind() != reflect.Pointer {
		if !value.CanAddr() {
			return nil
		}
		pointer = value.Addr()
	}
	if pointer.IsNil() || !pointer.CanInterface() {
		return nil
	}
	switch pointer.Type() {
	case reflect.TypeFor[*statefulDecoder](), reflect.TypeFor[*statefulEncoder]():
		return pointer.Interface()
	}
	return nil
}

// leaves adds what a decoder reads from: a string, or the results of a call
// as [flattenResults] lists them.
func (r *reached) leaves(value reflect.Value) {
	if !value.IsValid() {
		return
	}
	if value.Type() != reflect.TypeFor[[]reflect.Value]() {
		r.value(value)
		return
	}
	for i := range value.Len() {
		r.value(value.Index(i).Interface().(reflect.Value))
	}
}

// value adds the memory that value refers to outside itself, and what that
// refers to in turn. A channel can't be asked what it holds, see
// [handle.sink], and a function holds nothing a guest made.
func (r *reached) value(value reflect.Value) {
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.String:
		if value.Len() > 0 && r.first(value.Pointer()) {
			r.bytes += int64(value.Len())
		}
	case reflect.Slice:
		if value.Cap() == 0 || !r.first(value.Pointer()) {
			return
		}
		r.bytes += int64(value.Cap()) * int64(value.Type().Elem().Size())
		r.elements(value)
	case reflect.Array:
		if !refers(value.Type().Elem()) {
			return
		}
		for i := range value.Len() {
			r.value(value.Index(i))
		}
	case reflect.Struct:
		// A reflect.Value is one of the bridge's own, not a guest's value.
		if value.Type() == reflect.TypeFor[reflect.Value]() {
			return
		}
		for i := range value.NumField() {
			r.value(value.Field(i))
		}
	case reflect.Pointer:
		if value.IsNil() || !r.first(value.Pointer()) {
			return
		}
		r.bytes += int64(value.Type().Elem().Size())
		r.value(value.Elem())
	case reflect.Interface:
		if !value.IsNil() {
			r.value(value.Elem())
		}
	case reflect.Map:
		if value.IsNil() || !r.first(value.Pointer()) {
			return
		}
		entry := int64(value.Type().Key().Size()) + int64(value.Type().Elem().Size())
		for entries := value.MapRange(); entries.Next(); {
			r.bytes += entry
			r.value(entries.Key())
			r.value(entries.Value())
		}
	}
}

// elements adds what the elements of a slice refer to, up to its capacity: a
// slice that was taken of another reaches every element after its own.
func (r *reached) elements(slice reflect.Value) {
	if !refers(slice.Type().Elem()) {
		return
	}
	whole := slice.Slice(0, slice.Cap())
	for i := range whole.Len() {
		r.value(whole.Index(i))
	}
}

// refers reports whether a value of the type can refer to memory outside
// itself, so that the bytes and numbers of a large slice are not walked.
func refers(rtype reflect.Type) bool {
	switch rtype.Kind() {
	case reflect.String, reflect.Slice, reflect.Pointer, reflect.Interface, reflect.Map:
		return true
	case reflect.Array:
		return refers(rtype.Elem())
	case reflect.Struct:
		for i := range rtype.NumField() {
			if refers(rtype.Field(i).Type) {
				return true
			}
		}
	}
	return false
}
