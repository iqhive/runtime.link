package jrpc

import (
	"cmp"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// field is one JSON member of a struct, which may live in an embedded struct.
type field struct {
	name      string
	tagged    bool
	index     []int
	typ       reflect.Type
	omitEmpty bool
	omitZero  bool
	quoted    bool
}

// zeroer is what the omitzero option looks for before falling back on the zero value.
type zeroer interface{ IsZero() bool }

// structFields applies the rules of encoding/json, which keeps its version
// private: embedded structs are flattened breadth first, and a name claimed
// twice goes to the shallowest field, then to a tagged one, or to nobody.
func structFields(rtype reflect.Type) []field {
	var current []field
	next := []field{{typ: rtype}}
	var count, nextCount map[reflect.Type]int
	visited := map[reflect.Type]bool{}
	var fields []field
	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}
		for _, parent := range current {
			if visited[parent.typ] {
				continue
			}
			visited[parent.typ] = true
			for i := range parent.typ.NumField() {
				member := parent.typ.Field(i)
				if isHiddenField(member) {
					continue
				}
				tag := member.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, options, _ := strings.Cut(tag, ",")
				if !isValidTagName(name) {
					name = ""
				}
				index := append(slices.Clone(parent.index), i)
				target := member.Type
				if target.Name() == "" && target.Kind() == reflect.Pointer {
					target = target.Elem()
				}
				if name == "" && member.Anonymous && target.Kind() == reflect.Struct {
					nextCount[target]++
					if nextCount[target] == 1 {
						next = append(next, field{index: index, typ: target})
					}
					continue
				}
				found := field{
					name:      name,
					tagged:    name != "",
					index:     index,
					omitEmpty: hasTagOption(options, "omitempty"),
					omitZero:  hasTagOption(options, "omitzero"),
					quoted:    hasTagOption(options, "string") && isQuotableKind(target.Kind()),
				}
				if found.name == "" {
					found.name = member.Name
				}
				fields = append(fields, found)
				// A type embedded twice at one depth hides its own fields;
				// the second copy is what makes the fields below see a clash.
				if count[parent.typ] > 1 {
					fields = append(fields, found)
				}
			}
		}
	}
	return dominantFields(fields)
}

// layout saves working out a struct's fields again for every element of a slice.
func layout(known map[reflect.Type][]field, rtype reflect.Type) []field {
	fields, ok := known[rtype]
	if !ok {
		fields = structFields(rtype)
		known[rtype] = fields
	}
	return fields
}

func isHiddenField(member reflect.StructField) bool {
	if !member.Anonymous {
		return !member.IsExported()
	}
	target := member.Type
	if target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	return !member.IsExported() && target.Kind() != reflect.Struct
}

func hasTagOption(options, option string) bool {
	for candidate := range strings.SplitSeq(options, ",") {
		if candidate == option {
			return true
		}
	}
	return false
}

func isValidTagName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", char) {
			continue
		}
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

func isQuotableKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

// dominantFields returns the winner of each name, in declaration order.
func dominantFields(fields []field) []field {
	slices.SortFunc(fields, func(a, b field) int {
		if order := strings.Compare(a.name, b.name); order != 0 {
			return order
		}
		if order := cmp.Compare(len(a.index), len(b.index)); order != 0 {
			return order
		}
		if a.tagged != b.tagged {
			if a.tagged {
				return -1
			}
			return 1
		}
		return slices.Compare(a.index, b.index)
	})
	winners := fields[:0]
	for start, end := 0, 0; start < len(fields); start = end {
		for end = start + 1; end < len(fields) && fields[end].name == fields[start].name; end++ {
		}
		if isTiedName(fields[start:end]) {
			continue
		}
		winners = append(winners, fields[start])
	}
	slices.SortFunc(winners, func(a, b field) int { return slices.Compare(a.index, b.index) })
	return winners
}

func isTiedName(claimants []field) bool {
	return len(claimants) > 1 &&
		len(claimants[0].index) == len(claimants[1].index) &&
		claimants[0].tagged == claimants[1].tagged
}

// matchField prefers an exact name so that fields differing only by case stay
// reachable; otherwise the first field in declaration order that folds to key.
func matchField(fields []field, key string) *field {
	for i := range fields {
		if fields[i].name == key {
			return &fields[i]
		}
	}
	for i := range fields {
		if strings.EqualFold(fields[i].name, key) {
			return &fields[i]
		}
	}
	return nil
}

// valueIn reports false when a nil embedded pointer stands between rvalue and the field.
func (f *field) valueIn(rvalue reflect.Value) (reflect.Value, bool) {
	for _, i := range f.index {
		if rvalue.Kind() == reflect.Pointer {
			if rvalue.IsNil() {
				return reflect.Value{}, false
			}
			rvalue = rvalue.Elem()
		}
		rvalue = rvalue.Field(i)
	}
	return rvalue, true
}
