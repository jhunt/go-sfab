package main

import (
	"fmt"
	"os"
)

func (h *Hub) dumpState() {
	h.lock.Lock()
	defer h.lock.Unlock()

	for agent, ch := range h.agents {
		fmt.Fprintf(os.Stderr, "[%s] %v\n", agent, ch)
	}
}
