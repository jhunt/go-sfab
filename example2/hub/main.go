package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jhunt/go-log"
	"github.com/jhunt/go-sfab"
)

func pause(ms int) {
	fmt.Fprintf(os.Stderr, "waiting %dms for next job dispatch to (agent@example.com)...\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Fprintf(os.Stderr, "woke up; resuming job dispatch...\n")
}

func work(h *sfab.Hub, run int) {
	fmt.Fprintf(os.Stderr, "run %d: running command...\n", run)
	replies, err := h.Send("agent@example.com", []byte(fmt.Sprintf(`{"run":"%d"}`, run)), 2*time.Second)
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
	fmt.Fprintf(os.Stderr, "telling (agent@example.com) to exit...\n")

	replies, err := h.Send("agent@example.com", []byte(`/part`), 2*time.Second)
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

	key, err := sfab.PrivateKeyFromFile("id_rsa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load host private key: %s\n", err)
		os.Exit(1)
	}

	h := &sfab.Hub{
		Bind:      "127.0.0.1:5000",
		HostKey:   key,
		KeepAlive: 10 * time.Second,
	}
	//h.AuthorizeKeys("/Users/srini/go/src/github.com/jhunt/go-sfab/example2/agent/id_rsa.pub")

	run := 1
	go func() {
		for {
			for i := 0; i < 3; i++ {
				run += 1
				work(h, run)
				pause(200)
			}
			part(h)
			listActiveConnection := h.ActiveAgentConnection()
			for _, agentS := range listActiveConnection {
				fmt.Println("NAME: ", agentS.Name)
				fmt.Println("KEY: ", agentS.Key)
				fmt.Println("Status: ", agentS.Status)
			}
			pause(500)
			if len(listActiveConnection) != 0 {
				fmt.Println("OPERATOR AUTHORIZING AGENT")
				agent := listActiveConnection[0]
				h.KeyMasterAuth(agent.Name, agent.Key, true)
				fmt.Println("OPERATOR AUTHORIZING AGENT SUCCESSFUL")
			}
		}
	}()

	if err := h.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %s\n", err)
	}
}
