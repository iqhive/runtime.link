package api

import (
	"os"

	"runtime.link/qnq"
)

func init() {
	qnq.RegisterHost(func(structure qnq.Structure) {
		if port := os.Getenv("PORT"); port != "" {
			if err := ListenAndServe(":"+port, nil, structure); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
				os.Exit(1)
			}
			os.Exit(0)
		}
	})
}
