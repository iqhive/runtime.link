package xyz_test

import (
	"bytes"
	"encoding"
	"encoding/gob"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

func TestSwitch(t *testing.T) {
	type Animal xyz.Switch[int, struct {
		Cat Animal
		Dog Animal
	}]
	var Animals = xyz.AccessorFor(Animal.Values)

	var animal = Animals.Cat

	if animal.String() != "Cat" {
		t.Fatal("unexpected value")
	}

	switch animal {
	case Animals.Cat:
	case Animals.Dog:
		t.Fatal("unexpected value")
	default:
		t.Fatal("unexpected value")
	}

	var decoded Animal
	if err := decoded.UnmarshalJSON([]byte(`"Dog"`)); err != nil {
		t.Fatal(err)
	}
	if decoded != Animals.Dog {
		t.Fatal("unexpected value")
	}
}

type status xyz.Switch[string, struct {
	Open status `json:"in progress"`
	Done status `json:"done"`
}]

var statuses = xyz.AccessorFor(status.Values)

func TestSwitchTextKeepsSpaces(t *testing.T) {
	text, err := statuses.Open.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var back status
	if err := back.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if back != statuses.Open {
		t.Errorf("got %q", back.Raw())
	}
	data, err := json.Marshal(map[status]int{statuses.Open: 1})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[status]int
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if keys[statuses.Open] != 1 {
		t.Errorf("%s decoded to %v", data, keys)
	}
	other := xyz.Raw[status]("not a case")
	if err := back.UnmarshalText([]byte(other.Raw())); err != nil || back != other {
		t.Errorf("got %q, %v", back.Raw(), err)
	}
	if err := back.UnmarshalText(nil); err != nil || back.Raw() != "" {
		t.Errorf("empty text left %q, %v", back.Raw(), err)
	}
}

func TestSwitchOverUintptr(t *testing.T) {
	type pointer xyz.Switch[uintptr, struct {
		A pointer
		B pointer
	}]
	Pointers := xyz.AccessorFor(pointer.Values)
	if Pointers.A == Pointers.B {
		t.Fatal("every case is equal")
	}
	if Pointers.A.Raw() != 0 || Pointers.B.Raw() != 1 {
		t.Errorf("A is %d, B is %d", Pointers.A.Raw(), Pointers.B.Raw())
	}
	if got := Pointers.B.String(); got != "B" {
		t.Errorf("String is %q", got)
	}
	var v pointer
	if err := json.Unmarshal([]byte(`"B"`), &v); err != nil {
		t.Fatal(err)
	}
	if v != Pointers.B {
		t.Errorf("got %d", v.Raw())
	}
	b, err := json.Marshal(Pointers.B)
	if err != nil || string(b) != `"B"` {
		t.Errorf("got %s, %v", b, err)
	}
}

func checkSwitchValues[V any, P interface {
	*V
	json.Unmarshaler
	UnmarshalText([]byte) error
}](t *testing.T, name string, values ...V) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		for _, in := range values {
			b, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			var out V
			if err := json.Unmarshal(b, &out); err != nil {
				t.Errorf("%s: %v", b, err)
				continue
			}
			if !reflect.DeepEqual(out, in) {
				t.Errorf("%s decoded to %v, want %v", b, out, in)
			}
			text, err := any(in).(encoding.TextMarshaler).MarshalText()
			if err != nil {
				t.Fatal(err)
			}
			var back V
			if err := P(&back).UnmarshalText(text); err != nil {
				t.Errorf("%s: %v", text, err)
				continue
			}
			if !reflect.DeepEqual(back, in) {
				t.Errorf("%s decoded to %v, want %v", text, back, in)
			}
		}
	})
}

func TestEveryStorageOfASwitchRoundTrips(t *testing.T) {
	type (
		ints     xyz.Switch[int, struct{ A, B ints }]
		int8s    xyz.Switch[int8, struct{ A, B int8s }]
		int16s   xyz.Switch[int16, struct{ A, B int16s }]
		int32s   xyz.Switch[int32, struct{ A, B int32s }]
		int64s   xyz.Switch[int64, struct{ A, B int64s }]
		uints    xyz.Switch[uint, struct{ A, B uints }]
		uint8s   xyz.Switch[uint8, struct{ A, B uint8s }]
		uint16s  xyz.Switch[uint16, struct{ A, B uint16s }]
		uint32s  xyz.Switch[uint32, struct{ A, B uint32s }]
		uint64s  xyz.Switch[uint64, struct{ A, B uint64s }]
		uptrs    xyz.Switch[uintptr, struct{ A, B uptrs }]
		float32s xyz.Switch[float32, struct{ A, B float32s }]
		float64s xyz.Switch[float64, struct{ A, B float64s }]
		strs     xyz.Switch[string, struct{ A, B strs }]
		bools    xyz.Switch[bool, struct{ No, Yes bools }]
	)
	checkSwitchValues(t, "int", xyz.Raw[ints](0), xyz.Raw[ints](1), xyz.Raw[ints](99), xyz.Raw[ints](-3))
	checkSwitchValues(t, "int8", xyz.Raw[int8s](0), xyz.Raw[int8s](1), xyz.Raw[int8s](99), xyz.Raw[int8s](-3))
	checkSwitchValues(t, "int16", xyz.Raw[int16s](1), xyz.Raw[int16s](99))
	checkSwitchValues(t, "int32", xyz.Raw[int32s](1), xyz.Raw[int32s](99))
	checkSwitchValues(t, "int64", xyz.Raw[int64s](1), xyz.Raw[int64s](99))
	checkSwitchValues(t, "uint", xyz.Raw[uints](1), xyz.Raw[uints](99))
	checkSwitchValues(t, "uint8", xyz.Raw[uint8s](1), xyz.Raw[uint8s](99))
	checkSwitchValues(t, "uint16", xyz.Raw[uint16s](1), xyz.Raw[uint16s](99))
	checkSwitchValues(t, "uint32", xyz.Raw[uint32s](1), xyz.Raw[uint32s](99))
	checkSwitchValues(t, "uint64", xyz.Raw[uint64s](1), xyz.Raw[uint64s](99))
	checkSwitchValues(t, "uintptr", xyz.Raw[uptrs](1), xyz.Raw[uptrs](99))
	checkSwitchValues(t, "float32", xyz.Raw[float32s](1), xyz.Raw[float32s](1.5))
	checkSwitchValues(t, "float64", xyz.Raw[float64s](1), xyz.Raw[float64s](1.5))
	checkSwitchValues(t, "string", xyz.Raw[strs]("A"), xyz.Raw[strs]("a b"), xyz.Raw[strs](""))
	checkSwitchValues(t, "bool", xyz.Raw[bools](true), xyz.Raw[bools](false))
}

func TestSwitchValueOutsideTheSetRoundTrips(t *testing.T) {
	type animal xyz.Switch[int, struct {
		Cat animal
		Dog animal
	}]
	v := xyz.Raw[animal](99)
	b, err := json.Marshal(v)
	if err != nil || string(b) != `"99"` {
		t.Fatalf("got %s, %v", b, err)
	}
	var back animal
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back != v {
		t.Errorf("got %d", back.Raw())
	}
	if err := json.Unmarshal([]byte(`99`), &back); err != nil || back != v {
		t.Errorf("a plain number: %d, %v", back.Raw(), err)
	}
}

func TestSwitchRejectsWhatIsNotAValue(t *testing.T) {
	type animal xyz.Switch[int, struct {
		Cat animal
		Dog animal
	}]
	Animals := xyz.AccessorFor(animal.Values)
	for _, input := range []string{`"bogus"`, `"5x"`, `"5 6"`, `"0x10"`, `" 7"`, `""`, `{}`, `[1]`, `1.5`} {
		v := Animals.Dog
		if err := json.Unmarshal([]byte(input), &v); err == nil {
			t.Errorf("%s decoded to %d", input, v.Raw())
		}
		if v != Animals.Dog {
			t.Errorf("%s changed the value to %d", input, v.Raw())
		}
	}
	v := Animals.Dog
	if err := v.UnmarshalText([]byte("bogus")); err == nil {
		t.Errorf("bogus decoded to %d", v.Raw())
	}
	if v != Animals.Dog {
		t.Errorf("bogus changed the value to %d", v.Raw())
	}
}

// Text is read with fmt.Sscan, as it was in 840f71d: it takes any form of a
// number that fmt reads, and leaves out what follows the number.
func TestSwitchTextIsScanned(t *testing.T) {
	type animal xyz.Switch[int, struct {
		Cat animal
		Dog animal
	}]
	for input, want := range map[string]int{"5x": 5, "5 6": 5, "0x10": 16, " 7": 7, "Dog": 1} {
		var v animal
		if err := v.UnmarshalText([]byte(input)); err != nil || v.Raw() != want {
			t.Errorf("%q decoded to %d, %v, want %d", input, v.Raw(), err, want)
		}
	}
	v := xyz.Raw[animal](3)
	if err := v.UnmarshalText(nil); err != nil || v.Raw() != 3 {
		t.Errorf("empty text left %d, %v", v.Raw(), err)
	}
}

func TestSwitchUnmarshalOfNothing(t *testing.T) {
	var v status
	if err := v.UnmarshalJSON(nil); err == nil {
		t.Error("nothing decoded")
	}
	type animal xyz.Switch[int, struct{ Cat, Dog animal }]
	var a animal
	if err := a.UnmarshalJSON(nil); err == nil {
		t.Error("nothing decoded")
	}
}

// The name of a case is its whole json tag, options and all, as in 840f71d,
// where it is in callers' data.
func TestSwitchNameIsItsWholeTag(t *testing.T) {
	type tagged xyz.Switch[string, struct {
		Named   tagged `json:"named,omitempty"`
		Options tagged `json:",omitempty"`
		Empty   tagged `json:""`
	}]
	Taggeds := xyz.AccessorFor(tagged.Values)
	for want, got := range map[string]tagged{"named,omitempty": Taggeds.Named, ",omitempty": Taggeds.Options, "": Taggeds.Empty} {
		if got.Raw() != want {
			t.Errorf("got %q, want %q", got.Raw(), want)
		}
	}
	type numbered xyz.Switch[int, struct {
		A numbered `json:"first,omitempty"`
		B numbered `json:",omitempty"`
	}]
	Numbereds := xyz.AccessorFor(numbered.Values)
	if got := Numbereds.A.String(); got != "first,omitempty" {
		t.Errorf("got %q", got)
	}
	if got := Numbereds.B.String(); got != ",omitempty" {
		t.Errorf("got %q", got)
	}
	b, err := json.Marshal(Numbereds.A)
	if err != nil || string(b) != `"first,omitempty"` {
		t.Errorf("got %s, %v", b, err)
	}
	var back numbered
	if err := json.Unmarshal(b, &back); err != nil || back != Numbereds.A {
		t.Errorf("got %d, %v", back.Raw(), err)
	}
	values := Numbereds.A.ValuesJSON()
	if len(values) != 2 || string(values[0]) != `"first,omitempty"` || string(values[1]) != `",omitempty"` {
		t.Errorf("ValuesJSON is %q", values)
	}
}

func TestSwitchAccessors(t *testing.T) {
	type animal xyz.Switch[int, struct {
		Cat animal `json:"cat"`
		Dog animal `json:"dog"`
	}]
	Animals := xyz.AccessorFor(animal.Values)
	dog := Animals.Dog
	if dog.Raw() != 1 || dog.Interface() != 1 {
		t.Errorf("Raw is %d, Interface is %v", dog.Raw(), dog.Interface())
	}
	*dog.RawPointer() = 0
	if dog != Animals.Cat {
		t.Errorf("RawPointer wrote %d", dog.Raw())
	}
	*dog.InterfaceAddr().(*int) = 1
	if dog != Animals.Dog {
		t.Errorf("InterfaceAddr wrote %d", dog.Raw())
	}
	values := dog.ValuesJSON()
	if len(values) != 2 || string(values[0]) != `"cat"` || string(values[1]) != `"dog"` {
		t.Errorf("ValuesJSON is %q", values)
	}
}

func TestSwitchGobRoundTrip(t *testing.T) {
	type animal xyz.Switch[int, struct {
		Cat animal
		Dog animal
	}]
	Animals := xyz.AccessorFor(animal.Values)
	type holder struct {
		Pet animal
	}
	var wire bytes.Buffer
	if err := gob.NewEncoder(&wire).Encode(holder{Animals.Dog}); err != nil {
		t.Fatal(err)
	}
	var back holder
	if err := gob.NewDecoder(&wire).Decode(&back); err != nil {
		t.Fatal(err)
	}
	if back.Pet != Animals.Dog {
		t.Errorf("got %d", back.Pet.Raw())
	}
	b, err := statuses.Open.GobEncode()
	if err != nil {
		t.Fatal(err)
	}
	var text status
	if err := text.GobDecode(b); err != nil {
		t.Fatal(err)
	}
	if text != statuses.Open {
		t.Errorf("got %q", text.Raw())
	}
}
