package main

import (
	"strings"
	"fmt"
)

// A MessageType identifies what the remote end is expectd
// to do with a given message.
//
// The following Message Types are defined:
//
//   AgentExec - The Hub would like the Agent to run something,
//               and report back with the output of the process.
//
//    HubInfo  - The Hub would like to communicate some piece
//               of information (server details, time of day,
//               whatever) to the Agent, without requiring a
//               response.
//
type MessageType string

const (
	AgentExec MessageType = "run"
	HubInfo               = "info"
)

// A Message is sent to an Agent when the Hub wishes to
// execute something remotely.
//
type Message struct {
	// What type of message is this?
	Type MessageType

	// The raw payload of the message.  This is opaque
	// to sFAB; the library merely passes these bytes
	// off to the Agent, whose implementation determines
	// what to do with them.
	//
	data []byte
}

func (m Message) Text() string {
	return string(m.data)
}

func (m Message) Bytes() []byte {
	return m.data
}

func (m Message) String() string {
	return fmt.Sprintf("%s: %s\n", m.Type, m.Text())
}

func ParseMessage(s string) (Message, error) {
	types := []MessageType{AgentExec, HubInfo}

	for _, t := range types {
		if strings.HasPrefix(s, string(t)+": ") {
			return Message{
				Type: t,
				data: []byte(strings.TrimPrefix(s, string(t)+": ")),
			}, nil
		}
	}

	return Message{}, fmt.Errorf("unrecognized message type")
}
