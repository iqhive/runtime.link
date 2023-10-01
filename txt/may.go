package txt

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type (
	MayPrefix  bool
	MayContain string
)

func (p MayPrefix) String() string { return strconv.FormatBool(bool(p)) }

func (p MayPrefix) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	if strings.HasPrefix(raw, string(tag)) {
		*(ptr.(*MayPrefix)) = true
		return len(tag), nil
	}
	*(ptr.(*MayPrefix)) = false
	return 0, nil
}

func (c MayContain) String() string { return string(c) }
func (MayContain) MatchString(ptr any, raw string, tag reflect.StructTag) (n int, err error) {
	contains := ptr.(*MayContain)
	for _, char := range raw {
		if !strings.ContainsRune(string(tag), char) {
			return 0, fmt.Errorf("invalid '%v' character", string(char))
		}
		*contains += MayContain(char)
	}
	return 0, nil
}
