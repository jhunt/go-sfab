package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Keys() {
	resp, err := http.Get(fmt.Sprintf("%s/keys", opts.Keys.API))
	if err != nil {
		fmt.Errorf("No response from HUB")
	}
	defer resp.Body.Close()
	keys, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", keys)
}
