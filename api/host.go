package api

import (
	"errors"
	"os"

	"runtime.link/qnq"
)

func init() {
	qnq.RegisterHost(func(structure qnq.Structure) {
		if len(os.Args) > 1 {
			if err := host(structure); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
			}
			os.Exit(0)
		}
	})
}

func host(structure qnq.Structure) error {
	return errors.New("not implemented")
}
