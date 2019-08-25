package main

import (
	"fmt"
	"os"
	"time"
)

func Server(args []string) {
	h := &Hub{
		Bind:         "127.0.0.1:4771",
		HostKeyFile:  "host_key",
		AuthKeysFile: "id_rsa.pub",
	}

	run := 1
	go func() {
		for {
			if run%5 == 0 {
				h.dumpState()
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
	h.Listen()
}
