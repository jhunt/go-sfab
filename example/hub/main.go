package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jhunt/go-log"
	"github.com/jhunt/go-sfab"
)

func pause(ms int) {
	fmt.Fprintf(os.Stderr, "waiting %dms for next job dispatch to (some-agent)...\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Fprintf(os.Stderr, "woke up; resuming job dispatch...\n")
}

func work(h *sfab.Hub, run int) {
	fmt.Fprintf(os.Stderr, "run %d: running command...\n", run)
	replies, err := h.Send("some-agent", []byte(fmt.Sprintf(`{"run":"%d"}`, run)), 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uh-oh: %s\n", err)

	} else {
		fmt.Fprintf(os.Stderr, "run %d: command running...\n", run)

		go func() {
			for r := range replies {
				if r.IsStdout() {
					fmt.Printf("run %d STDOUT | %s\n", run, r.Text())
				} else if r.IsStderr() {
					fmt.Printf("run %d STDERR | %s\n", run, r.Text())
				} else if r.IsExit() {
					fmt.Printf("run %d EXIT   | command exited %d\n", run, r.ExitCode())
				} else if r.IsError() {
					fmt.Printf("run %d ERROR  | %s\n", run, r.Error())
				}
			}
		}()
	}
}

func part(h *sfab.Hub) {
	fmt.Fprintf(os.Stderr, "telling (some-agent) to exit...\n")

	replies, err := h.Send("some-agent", []byte(`/part`), 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uh-oh: %s\n", err)
	}

	go h.IgnoreReplies(replies)
}

func main() {
	log.SetupLogging(log.LogConfig{
		Type:  "console",
		Level: "debug",
	})

	key, err := sfab.PrivateKeyFromFile("host_key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load host private key: %s\n", err)
		os.Exit(1)
	}
	h := &sfab.Hub{
		Bind:      "127.0.0.1:4771",
		HostKey:   key,
		KeepAlive: 10 * time.Second,
	}

	if err := h.AuthorizeKeys("id_rsa.pub"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to authorize keys: %s\n", err)
		os.Exit(1)
	}

	run := 1
	go func() {
		for {
			for i := 0; i < 3; i++ {
				run += 1
				work(h, run)
				pause(200)
			}
			part(h)
			pause(500)
		}
	}()

	if err := h.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %s\n", err)
	}
}
