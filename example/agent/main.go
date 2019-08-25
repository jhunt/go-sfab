package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jhunt/go-sfab"
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

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "USAGE:%s AGENT-ID\n", os.Args[0])
		os.Exit(1)
	}

	a := sfab.Agent{
		Identity:       os.Args[1],
		Timeout:        30 * time.Second,
		PrivateKeyFile: "example/id_rsa",
	}

	err := a.Connect("tcp4", "127.0.0.1:4771", jsonnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}
}
