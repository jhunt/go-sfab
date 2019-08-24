package main

import (
	"os"
	"fmt"
)

func bail(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}
}
