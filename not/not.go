package not

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type (
	Prefix bool
	Suffix bool
	Middle bool

	Only bool
)

func (mid Middle) String() string { return strconv.FormatBool(bool(mid)) }
func (mid Middle) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.Contains(raw, string(tag)) && !strings.HasPrefix(raw, string(tag)) && !strings.HasSuffix(raw, string(tag)) {
		*(ptr.(*Middle)) = false
		return 0, fmt.Errorf("invalid '%v' middle", mid)
	}
	*(ptr.(*Middle)) = true
	return 0, nil
}

func (pfx Prefix) String() string { return strconv.FormatBool(bool(pfx)) }
func (pfx Prefix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasPrefix(raw, string(tag)) {
		*(ptr.(*Prefix)) = false
		return 0, fmt.Errorf("invalid '%v' prefix", pfx)
	}
	*(ptr.(*Prefix)) = true
	return 0, nil
}

func (sfx Suffix) String() string { return strconv.FormatBool(bool(sfx)) }
func (sfx Suffix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasSuffix(raw, string(tag)) {
		*(ptr.(*Suffix)) = false
		return 0, fmt.Errorf("invalid '%v' suffix", sfx)
	}
	*(ptr.(*Suffix)) = true
	return 0, nil
}

func (oly Only) String() string { return strconv.FormatBool(bool(oly)) }
func (oly Only) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	var only = true
	for _, char := range raw {
		if strings.ContainsRune(string(tag), char) {
			continue
		}
		only = false
	}
	if only {
		*(ptr.(*Only)) = false
		return 0, fmt.Errorf("invalid '%v' characters", oly)
	}
	*(ptr.(*Only)) = true
	return 0, nil
}
