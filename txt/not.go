package txt

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type (
	NotPrefix bool
	NotSuffix bool
	NotMiddle bool

	NotOnly bool
)

func (mid NotMiddle) String() string { return strconv.FormatBool(bool(mid)) }
func (mid NotMiddle) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.Contains(raw, string(tag)) && !strings.HasPrefix(raw, string(tag)) && !strings.HasSuffix(raw, string(tag)) {
		*(ptr.(*NotMiddle)) = false
		return 0, fmt.Errorf("invalid '%v' middle", mid)
	}
	*(ptr.(*NotMiddle)) = true
	return 0, nil
}

func (pfx NotPrefix) String() string { return strconv.FormatBool(bool(pfx)) }
func (pfx NotPrefix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasPrefix(raw, string(tag)) {
		*(ptr.(*NotPrefix)) = false
		return 0, fmt.Errorf("invalid '%v' prefix", pfx)
	}
	*(ptr.(*NotPrefix)) = true
	return 0, nil
}

func (sfx NotSuffix) String() string { return strconv.FormatBool(bool(sfx)) }
func (sfx NotSuffix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasSuffix(raw, string(tag)) {
		*(ptr.(*NotSuffix)) = false
		return 0, fmt.Errorf("invalid '%v' suffix", sfx)
	}
	*(ptr.(*NotSuffix)) = true
	return 0, nil
}

func (oly NotOnly) String() string { return strconv.FormatBool(bool(oly)) }
func (oly NotOnly) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	var only = true
	for _, char := range raw {
		if strings.ContainsRune(string(tag), char) {
			continue
		}
		only = false
	}
	if only {
		*(ptr.(*NotOnly)) = false
		return 0, fmt.Errorf("invalid '%v' characters", oly)
	}
	*(ptr.(*NotOnly)) = true
	return 0, nil
}
