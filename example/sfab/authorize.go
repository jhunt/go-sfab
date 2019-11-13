package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Authorize() {
	requestBody, err := json.Marshal(map[string]string{
		"Identity":    opts.Authorize.Name,
		"Fingerprint": opts.Authorize.KeyFingerprint,
	})

	resp, err := http.Post(fmt.Sprintf("%s/authz", opts.Authorize.API), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Errorf("Could not authorize agent: %s", opts.Authorize.Name)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("RESP: %s\n", body)
}
