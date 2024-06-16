package main

import (
	"fmt"
	"os"

	"runtime.link/zgo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go [build/run]")
		return
	}
	switch os.Args[1] {
	case "build":
		if err := zgo.Build(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "run":
		if err := zgo.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: go [build/run]")
		os.Exit(1)
	}
}
