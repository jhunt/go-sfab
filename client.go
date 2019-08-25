package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

func jsonnet(cmd []byte, out io.Writer) {
	var x map[string]interface{}
	err := json.Unmarshal(cmd, &x)
	if err != nil {
		fmt.Fprintf(out, "ERROR: %s\n", err)
		return
	}

	b, err := json.MarshalIndent(x, "json >> ", " . ")
	if err != nil {
		fmt.Fprintf(out, "ERROR: %s\n", err)
		return
	}

	output := func(f string, args ...interface{}) {
		fmt.Fprintf(out, f, args...)
		fmt.Fprintf(os.Stderr, f, args...)
	}
	output("-----------------[ output ]-------------\n")
	output("json >> %s\n", string(b))
	output("-----------------[ ====== ]-------------\n\n")
}

func Client(args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "USAGE: go-sfab client AGENT-ID\n")
		os.Exit(1)
	}

	a := Agent{
		Identity:       args[0],
		Timeout:        30 * time.Second,
		PrivateKeyFile: "id_rsa",
	}

	err := a.Connect("tcp4", "127.0.0.1:4771", jsonnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}
}
