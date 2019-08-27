package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jhunt/go-log"
	"github.com/jhunt/go-sfab"
)

func main() {
	log.SetupLogging(log.LogConfig{
		Type:  "console",
		Level: "debug",
	})

	key, err := sfab.PrivateKeyFromFile("example/host_key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load host private key: %s\n", err)
		os.Exit(1)
	}
	h := &sfab.Hub{
		Bind:      "127.0.0.1:4771",
		HostKey:   key,
		KeepAlive: 10 * time.Second,
	}

	if err := h.AuthorizeKeys("example/id_rsa.pub"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to authorize keys: %s\n", err)
		os.Exit(1)
	}

	run := 1
	go func() {
		for {
			if run%100 == 0 {
				fmt.Fprintf(os.Stderr, "telling (some-agent) to exit...\n")
				replies, err := h.Send("some-agent", []byte(fmt.Sprintf(`/part`, run)))
				if err != nil {
					fmt.Fprintf(os.Stderr, "uh-oh: %s\n", err)
				}
				go func() {
					for range replies {
					}
				}()
			}

			if run%5 == 0 {
				fmt.Fprintf(os.Stderr, "waiting 5s for next job dispatch to (some-agent)...\n")
				time.Sleep(5 * time.Second)
				fmt.Fprintf(os.Stderr, "woke up; resuming job dispatch...\n")
			}

			fmt.Fprintf(os.Stderr, "run %d: running command...\n", run)
			replies, err := h.Send("some-agent", []byte(fmt.Sprintf(`{"run":"%d"}`, run)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "uh-oh: %s\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "run %d: command running...\n", run)
				go func(run int) {
					for r := range replies {
						if r.IsStdout() {
							fmt.Printf("run %d STDOUT | %s\n", run, r.Text())
						} else if r.IsStderr() {
							fmt.Printf("run %d STDERR | %s\n", run, r.Text())
						} else if r.IsExit() {
							fmt.Printf("run %d EXIT   | command exited %d\n", run, r.ExitCode())
						}
					}
				}(run)
			}
			run += 1
		}
	}()

	if err := h.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %s\n", err)
	}
}
