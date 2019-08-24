package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "client":
			Client(os.Args[2:])
			os.Exit(0)
		case "server":
			Server(os.Args[2:])
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "USAGE: %s (client|server) ADDRESS\n", os.Args[0])
	os.Exit(1)
}
