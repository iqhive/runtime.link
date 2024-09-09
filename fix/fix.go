package fix

import (
	"fmt"

	"runtime.link/api/xray"
)

type literal string

func Me(format literal, args ...any) error {
	return xray.Error(fmt.Errorf(string(format), args...), 1)
}

func This(err error) error { return xray.Error(err, 1) }
