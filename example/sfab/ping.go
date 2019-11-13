package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Ping() {
	requestBody, err := json.Marshal(map[string]string{
		"Identity":    opts.Ping.Name,
		"Fingerprint": opts.Ping.KeyFingerprint,
	})

	resp, err := http.Post(fmt.Sprintf("%s/ping", opts.Ping.API), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Errorf("Could not authorize agent: %s", opts.Ping.Name)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("RESP: %s\n", body)
}
