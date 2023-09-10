package lib

import (
	"errors"
	"os"

	"runtime.link/std"
)

func init() {
	std.RegisterHost(func(structure std.Structure) {
		if dir := os.Getenv("LIB_OUT"); dir != "" {
			if err := host(structure, dir); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
			}
		}
	})
}

func host(structure std.Structure, dir string) error {
	return errors.New("not implemented")
}
