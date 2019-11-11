package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"encoding/json"

	"github.com/jhunt/go-sfab"
)

func Hub() {
	key, err := sfab.PrivateKeyFromFile(opts.Hub.Key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load host private key from %s: %s\n", opts.Hub.Key, err)
		os.Exit(1)
	}
	h := &sfab.Hub{
		Bind:      opts.Hub.Bind,
		HostKey:   key,
		KeepAlive: time.Duration(opts.Hub.KeepAlive) * time.Second,
	}

	go func() {
		if err := h.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "listen: %s\n", err)
			os.Exit(1)
		}
	}()

	http.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Bad Request\n")
			return
		}

		authz := h.Authorizations()
		b, err := json.Marshal(authz)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal Server Failure\n")
			return
		}

		w.WriteHeader(200)
		fmt.Fprintf(w, "%s\n", string(b))
	})

	http.HandleFunc("/authz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Bad Request\n")
			return
		}

		defer r.Body.Close()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal Server Failure\n")
			return
		}

		var in struct {
			Identity    string
			Fingerprint string
		}
		if err := json.Unmarshal(b, &in); err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal Server Failure\n")
			return
		}

		for _, authz := range h.Authorizations() {
			if authz.Identity == in.Identity && authz.KeyFingerprint == in.Fingerprint {
				h.AuthorizeKey(in.Identity, authz.PublicKey)
				break
			}
		}

		w.WriteHeader(200)
		fmt.Fprintf(w, "OK!\n")
	})

	if err := http.ListenAndServe(opts.Hub.API, nil); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}
