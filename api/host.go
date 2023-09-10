package api

import (
	"errors"
	"os"

	"runtime.link/std"
)

func init() {
	std.RegisterHost(func(structure std.Structure) {
		if len(os.Args) > 1 {
			if err := host(structure); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
			}
		}
	})
}

func host(structure std.Structure) error {
	return errors.New("not implemented")
}
