package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jhunt/go-log"
	"github.com/jhunt/go-sfab"
)

func jsonnet(cmd []byte, stdout io.Writer, stderr io.Writer) (int, error) {
	var x map[string]interface{}

	fmt.Fprintf(stderr, "debug:: unmarshaling payload [%s]...\n", string(cmd))
	fmt.Fprintf(stderr, "debug::   if you prefer hex: [% x]...\n", cmd)
	err := json.Unmarshal(cmd, &x)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: %s\n", err)
		return 1, err
	}

	b, err := json.MarshalIndent(x, "json >> ", " . ")
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: %s\n", err)
		return 1, err
	}

	write := func(f string, args ...interface{}) {
		fmt.Fprintf(stdout, f, args...)
		fmt.Fprintf(os.Stderr, f, args...)
	}
	write("-----------------[ output ]-------------\n")
	write("json >> %s\n", string(b))
	write("-----------------[ ====== ]-------------\n\n")

	fmt.Fprintf(stderr, "debug:: exiting 0\n")
	return 0, nil
}

func main() {
	log.SetupLogging(log.LogConfig{
		Type:  "console",
		Level: "debug",
	})

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "USAGE:%s AGENT-ID\n", os.Args[0])
		os.Exit(1)
	}

	key, err := sfab.PrivateKeyFromFile("example/id_rsa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load agent private key: %s\n", err)
		os.Exit(1)
	}
	a := sfab.Agent{
		Identity:   os.Args[1],
		Timeout:    30 * time.Second,
		PrivateKey: key,
	}

	if err := a.Connect("tcp4", "127.0.0.1:4771", jsonnet); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}
}
