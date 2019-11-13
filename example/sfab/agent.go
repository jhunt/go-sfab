package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jhunt/go-sfab"
)

func jsonnet(cmd []byte, stdout io.Writer, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "debug:: unmarshaling payload [%s]...\n", string(cmd))
	fmt.Fprintf(stderr, "debug::   if you prefer hex: [% x]...\n", cmd)

	write := func(f string, args ...interface{}) {
		fmt.Fprintf(stdout, f, args...)
		fmt.Fprintf(os.Stderr, f, args...)
	}

	if string(cmd) == "ping" {
		write("PONG,%s", time.Now())
		fmt.Printf("PING  at time: %s\n", time.Now())
	}

	return 0, nil
}

func Agent() {
	key, err := sfab.PrivateKeyFromFile(opts.Agent.Key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load agent private key: %s\n", err)
		os.Exit(1)
	}

	a := sfab.Agent{
		Identity:   opts.Agent.Name,
		Timeout:    30 * time.Second,
		PrivateKey: key,
	}

	if err := a.Connect("tcp4", opts.Agent.Hub, jsonnet); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}
}
