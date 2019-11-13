package main

import (
	"time"
	"io"
	"fmt"
	"os"
	"net/http"
	"encoding/json"
	"bytes"
	"io/ioutil"
)

func get(url string) (string, error) {
	r, err := http.Get(fmt.Sprintf("%s%s", opts.API, url))
	if err != nil {
		return "", err
	}

	/* FIXME: check status code */

	defer r.Body.Close()
	out, err := ioutil.ReadAll(r.Body)
	return string(out), err
}

func post(url string, data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	r, err := http.Post(fmt.Sprintf("%s%s", opts.API, url), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}

	/* FIXME: check status code */

	defer r.Body.Close()
	out, err := ioutil.ReadAll(r.Body)
	return string(out), err
}

func bail(err error, msg string, args ...interface{}) {
	if err != nil {
		fmt.Fprintf(os.Stderr, msg, args...)
		fmt.Fprintf(os.Stderr, ": %s\n", err)
		os.Exit(1)
	}
}

func jsonnet(cmd []byte, stdout io.Writer, stderr io.Writer) (int, error) {
	fmt.Fprintf(stderr, "debug:: unmarshaling payload [%s]...\n", string(cmd))
	fmt.Fprintf(stderr, "debug::   if you prefer hex: [% x]...\n", cmd)

	write := func(f string, args ...interface{}) {
		fmt.Fprintf(stdout, f, args...)
		fmt.Fprintf(os.Stderr, f, args...)
	}

	if string(cmd) == "ping" {
		write("PONG,%s", time.Now())
		fmt.Printf("PING  at time: %s\n", time.Now())
	}

	return 0, nil
}
