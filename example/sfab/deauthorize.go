package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Deauthorize() {
	requestBody, err := json.Marshal(map[string]string{
		"Identity":    opts.Deauthorize.Name,
		"Fingerprint": opts.Deauthorize.KeyFingerprint,
	})

	resp, err := http.Post(fmt.Sprintf("%s/deauthz", opts.Deauthorize.API), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Errorf("Could not authorize agent: %s", opts.Deauthorize.Name)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("RESP: %s\n", body)
}
