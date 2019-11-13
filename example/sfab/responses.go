package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Responses() {
	resp, err := http.Get(fmt.Sprintf("%s/responses", opts.Responses.API))
	if err != nil {
		fmt.Errorf("No response from HUB")
	}
	keys, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", keys)
}
