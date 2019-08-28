package sfab

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

// A status wraps up the possible result of a
// remote execution; either it exits normally,
// with a Unix-style return code (code), or it
// breaks in new and exciting ways (err).
//
type status struct {
	code int
	err  error
}

// A session handles the interaction of a single
// remote execution.  This was broken out into its
// own object to enable parallel execution across
// multiple sessions multiplexed onto a single
// connection.
//
type session struct {
	// The connection that spawned us.

	connection *connection

	// The underlying I/O stream for writing our
	// standard output and standard error to.
	//
	channel ssh.Channel

	// The session requests channel, which allows
	// the remote end to communicate "out-of-band"
	// information like exit status / signal details.
	//
	requests <-chan *ssh.Request

	// A channel we will listen to (in finish()) for
	// the ultimate exit status (code or error) of
	// the remote execution.
	//
	exit chan status
}

// serviceRequests (which ought to be run in a
// goroutine) handles out-of-band requests from
// the remote Agent, and specifically captures
// the details of the "exit-status" (normal exit)
// and "exit-signal" (abnormal exit) requests.
//
func (s *session) serviceRequests() {
	for r := range s.requests {
		switch r.Type {
		case "exit-status":
			s.exit <- status{code: int(binary.BigEndian.Uint32(r.Payload))}
			return

		case "exit-signal":
			var sig struct {
				Signal     string
				CoreDumped bool
				Error      string
				Lang       string
			}
			if err := ssh.Unmarshal(r.Payload, &sig); err != nil {
				s.exit <- status{err: fmt.Errorf("failed to unmarshal SSH request: %s", err)}
			} else if sig.Signal != "" {
				s.exit <- status{err: fmt.Errorf("remote error (%s): %s", sig.Signal, sig.Error)}
			} else {
				s.exit <- status{err: fmt.Errorf("remote error: %s", sig.Error)}
			}
			return

		default:
			if r.WantReply {
				r.Reply(false, nil)
			}
		}
	}
}

// Starts the remote execution of the given payload.
// Mostly this just involves sending an "exec" request
// and handling failures (like broken pipes) sanely.
//
func (s *session) start(payload string) error {
	run := struct{ Command string }{payload}
	ok, err := s.channel.SendRequest("exec", true, ssh.Marshal(&run))
	if err == nil && !ok {
		err = fmt.Errorf("unspecified failure")
	}
	if err != nil {
		s.channel.Close()
		return err
	}

	return nil
}

// Finishes the remote execution of the given payload.
// This allows callers to interpose some babysitting
// logic between starting the exec, and handling all of
// the output and exit status stuff.
//
func (s *session) finish(reply chan *Response, reaper chan int) {
	var wg sync.WaitGroup
	wg.Add(2)
	go s.drain(&wg, fromStdout, s.channel, reply)
	go s.drain(&wg, fromStderr, s.channel.Stderr(), reply)
	s.channel.CloseWrite()

	var final *Response
	select {
	case rc := <-s.exit:
		go func() { <-reaper }()
		if rc.err != nil {
			final = &Response{
				from: fromError,
				err:  rc.err,
			}
		} else {
			final = &Response{
				from: fromExit,
				rc:   rc.code,
			}
		}

	case <-reaper:
		final = &Response{
			from: fromError,
			err:  fmt.Errorf("agent disconnected prematurely"),
		}
	}

	s.channel.Close()
	wg.Wait()

	reply <- final
	close(reply)
}

// Drains output from a given source, to a Response
// channel, and when the input is exhausted, fulfills
// the WaitGroup obligation passed in.
//
// (This mostly cleans up other code).
//
func (*session) drain(wg *sync.WaitGroup, whence from, in io.Reader, out chan *Response) {
	b := bufio.NewScanner(in)
	for b.Scan() {
		out <- &Response{
			from: whence,
			text: b.Text(),
		}
	}
	wg.Done()
}
