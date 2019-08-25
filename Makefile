build:
	go build .

examples:
	go build -race ./example/hub
	go build -race ./example/agent
