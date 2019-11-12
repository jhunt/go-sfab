package main

import (
	"fmt"
	"os"

	//	"github.com/jhunt/go-log"
	//	"github.com/jhunt/go-sfab"
	"github.com/jhunt/go-cli"
	env "github.com/jhunt/go-envirotron"
)

var opts struct {
	LogLevel string `cli:"-L, --log-level" env:"SFAB_LOG_LEVEL"`

	Hub struct {
		Bind string `cli:"-b, --bind" env:"SFAB_HUB_BIND"`
		API  string `cli:"-a, --api"  env:"SFAB_HUB_API"`
		Key  string `cli:"-k, --key"  env:"SFAB_HUB_HOST_KEY"`

		KeepAlive int `cli:"--keep-alive" env:"SFAB_HUB_KEEPALIVE"`
	} `cli:"hub"`

	Agent struct {
		Hub  string `cli:"-H, --hub"   env:"SFAB_HUB"`
		Key  string `cli:"-k, --key"   env:"SFAB_AGENT_KEY"`
		Name string `cli:"-n, --name" env:"SFAB_AGENT_NAME"`
	} `cli:"agent"`

	Keys struct {
		Hub string `cli:"-H, --hub"   env:"SFAB_HUB"`
	} `cli:"keys"`

	// USAGE: sfab ping -H ... agent@domain
	Ping struct {
		Hub string `cli:"-H, --hub"   env:"SFAB_HUB"`
	} `cli:"ping"`

	// USAGE: sfab authz -H ... agent@domain ke:yf:in:ge:rp:ri:nt
	Authorize struct {
		Hub string `cli:"-H, --hub"   env:"SFAB_HUB"`
	} `cli:"auth, authz, authorize"`

	// USAGE: sfab deauthz -H ... agent@domain ke:yf:in:ge:rp:ri:nt
	Deauthorize struct {
		Hub string `cli:"-H, --hub"   env:"SFAB_HUB"`
	} `cli:"deauth, deauthz, deauthorize"`
}

func main() {
	opts.LogLevel = "info"

	opts.Hub.Bind = "127.0.0.1:4771"
	opts.Hub.API = "127.0.0.1:4772"
	opts.Hub.KeepAlive = 10

	opts.Agent.Hub = "127.0.0.1:4771"

	env.Override(&opts)

	command, args, err := cli.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("command: '%s'\n", command)
	fmt.Printf("args:    [%v]\n", args)
	fmt.Printf("options: {%v}\n", opts)

	if command == "hub" {
		ok := true
		if opts.Hub.Key == "" {
			fmt.Fprintf(os.Stderr, "Missing required --key parameter (or SFAB_HUB_HOST_KEY environment variable)\n")
			ok = false
		}
		if opts.Hub.Bind == "" {
			fmt.Fprintf(os.Stderr, "Missing required --bind parameter (or SFAB_HUB_BIND environment variable)\n")
			ok = false
		}
		if opts.Hub.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		Hub()
	} else if command == "agent" {
		ok := true
		if opts.Agent.Hub == "" {
			fmt.Fprint(os.Stderr, "Missing required --hub parameter (or SFAB_HUB environment variable)\n")
			ok = false
		}
		if opts.Agent.Key == "" {
			fmt.Fprintf(os.Stderr, "Missing required --key parameter (or SFAB_AGENT_KEY environment variable)\n")
			ok = false
		}
		if opts.Agent.Name == "" {
			fmt.Fprintf(os.Stderr, "Missing required --name parameter (or SFAB_AGENT_NAME environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		Agent()
	} else {
		fmt.Fprintf(os.Stderr, "Command not recognized\n")
		os.Exit(2)
	}

	os.Exit(0)
}
