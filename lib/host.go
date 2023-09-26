package lib

import (
	"errors"
	"os"

	"runtime.link/qnq"
)

func init() {
	qnq.RegisterHost(func(structure qnq.Structure) {
		if dir := os.Getenv("LIB_OUT"); dir != "" {
			if err := host(structure, dir); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
			}
		}
	})
}

func host(structure qnq.Structure, dir string) error {
	return errors.New("not implemented")
}
