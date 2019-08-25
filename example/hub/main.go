package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jhunt/go-sfab"
)

func main() {
	h := &sfab.Hub{
		Bind:         "127.0.0.1:4771",
		HostKeyFile:  "example/host_key",
	}

	if err := h.AuthorizeKeys("example/id_rsa.pub"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to authorize keys: %s\n", err)
		os.Exit(1)
	}

	run := 1
	go func() {
		for {
			if run%5 == 0 {
				fmt.Fprintf(os.Stderr, "waiting 5s for next job dispatch to (some-agent)...\n")
				time.Sleep(5 * time.Second)
				fmt.Fprintf(os.Stderr, "woke up; resuming job dispatch...\n")
			}

			err := h.Send("some-agent", []byte(fmt.Sprintf(`{"run":"%d"}`, run)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "uh-oh: %s\n", err)
			}
			run += 1
		}
	}()

	if err := h.Listen(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %s\n", err)
	}
}
