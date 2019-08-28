package sfab

type from int

const (
	fromInit from = iota
	fromStdout
	fromStderr
	fromExit
	fromError
)

type Response struct {
	from from
	text string
	err  error
	rc   int
}

func (r Response) IsStdout() bool {
	return r.from == fromStdout
}

func (r Response) IsStderr() bool {
	return r.from == fromStderr
}

func (r Response) IsExit() bool {
	return r.from == fromExit
}

func (r Response) IsError() bool {
	return r.from == fromError
}

func (r Response) Text() string {
	return r.text
}

func (r Response) ExitCode() int {
	return r.rc
}

func (r Response) Error() error {
	return r.err
}

type Message struct {
	responses chan *Response
	payload   []byte
}
