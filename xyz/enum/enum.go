package enum

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// info records what is known about a registered enum type: the set of valid
// values (for membership tests) and the JSON-encoded value names in declaration
// order (for [ValuesJSON]).
type info struct {
	valid map[string]struct{}
	json  []json.RawMessage
}

// registry maps each registered enum's [reflect.Type] to its [info]. It is
// written only by [Register] (which runs during package initialisation) and
// read by [Validate] and [ValuesJSON].
var registry sync.Map // map[reflect.Type]info

// Register returns an accessor struct of type Cases, with each field set to a
// value derived from its field name (or json tag). Cases must be a struct whose
// fields are all of the enum type T (a named string type). Store the result in
// a package-level variable and use its fields as the enum's values.
//
//	type Animal string
//	var Animals = enum.Register[Animal, struct {
//		Dog Animal
//		Cat Animal
//	}]()
//
//	// Animals.Dog == Animal("Dog")
//
// Register also records the values of T so that [Validate] can report whether an
// arbitrary value of type T is one of them and [ValuesJSON] can list them.
func Register[T ~string, Cases any]() Cases {
	var values Cases
	rvalue := reflect.ValueOf(&values).Elem()
	rtype := rvalue.Type()
	entry := info{
		valid: make(map[string]struct{}, rtype.NumField()),
		json:  make([]json.RawMessage, 0, rtype.NumField()),
	}
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		name, ok := field.Tag.Lookup("json")
		if !ok {
			name = field.Name
		}
		rvalue.Field(i).SetString(name)
		entry.valid[name] = struct{}{}
		if b, err := json.Marshal(name); err == nil {
			entry.json = append(entry.json, b)
		}
	}
	registry.Store(reflect.TypeFor[T](), entry)
	return values
}

// Is reports whether val is a value of a registered enum type. Because enum
// types are plain named string types with no methods, this registry lookup is
// the only way to distinguish an enum from an ordinary string at runtime.
//
// A non-string value, or a string type that was never passed to [Register],
// reports false.
func Is(val any) bool {
	rvalue := reflect.ValueOf(val)
	if rvalue.Kind() != reflect.String {
		return false
	}
	_, ok := registry.Load(rvalue.Type())
	return ok
}

// Validate reports an error if val is not one of the known values of its enum
// type, as recorded by [Register]. It returns nil only when val's type has been
// registered and val is one of that type's values; an unregistered type (or a
// non-string value) is itself an error, since there is no known value set to
// validate it against.
func Validate(val any) error {
	rvalue := reflect.ValueOf(val)
	if rvalue.Kind() != reflect.String {
		return fmt.Errorf("%T is not a registered enum", val)
	}
	entry, ok := registry.Load(rvalue.Type())
	if !ok {
		return fmt.Errorf("%s is not a registered enum", rvalue.Type().String())
	}
	if _, ok := entry.(info).valid[rvalue.String()]; ok {
		return nil
	}
	return fmt.Errorf("%s: %q is not a valid value", rvalue.Type().String(), rvalue.String())
}

// ValuesJSON returns the JSON-encoded names of every value of val's enum type,
// in declaration order, as recorded by [Register]. It returns nil if val's type
// was never registered or is not a string type.
func ValuesJSON(val any) []json.RawMessage {
	rvalue := reflect.ValueOf(val)
	if rvalue.Kind() != reflect.String {
		return nil
	}
	entry, ok := registry.Load(rvalue.Type())
	if !ok {
		return nil
	}
	return entry.(info).json
}
