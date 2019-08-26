package sfab

type source int

const (
	stdoutSource source = iota
	stderrSource
	exitSource
)

type Response struct {
	source source
	text   string
	rc     int
}

func (r Response) IsStdout() bool {
	return r.source == stdoutSource
}

func (r Response) IsStderr() bool {
	return r.source == stderrSource
}

func (r Response) IsExit() bool {
	return r.source == exitSource
}

func (r Response) Text() string {
	return r.text
}

func (r Response) ExitCode() int {
	return r.rc
}

type Message struct {
	responses chan Response
	payload   []byte
}
