package main

import (
	"os"
	"time"

	fmt "github.com/jhunt/go-ansi"
	"github.com/jhunt/go-log"
	"github.com/jhunt/go-cli"
	env "github.com/jhunt/go-envirotron"

	"github.com/jhunt/go-sfab"
)

var opts struct {
	LogLevel string `cli:"-L, --log-level" env:"SFAB_LOG_LEVEL"`
	Help     bool   `cli:"-h, --help"`

	API string `cli:"-a, --api"  env:"SFAB_HUB_API"`

	Hub struct {
		Bind   string `cli:"-b, --bind"   env:"SFAB_HUB_BIND"`
		Listen string `cli:"-l, --listen" env:"SFAB_HUB_LISTEN"`
		Key    string `cli:"-k, --key"    env:"SFAB_HUB_HOST_KEY"`

		KeepAlive int `cli:"--keep-alive" env:"SFAB_HUB_KEEPALIVE"`
	} `cli:"hub"`

	Agent struct {
		Hub string `cli:"-H, --hub"   env:"SFAB_HUB"`
		Key string `cli:"-k, --key"   env:"SFAB_AGENT_KEY"`
	} `cli:"agent"`

	Keys      struct{} `cli:"keys"`
	Agents    struct{} `cli:"agents"`
	Responses struct{} `cli:"responses"`
	Ping      struct{} `cli:"ping"`

	Authorize struct {
		KeyFingerprint string `cli:"-f, --fingerprint" env:"SFAB_AGENT_FINGERPRINT"`
	} `cli:"auth, authz, authorize"`

	Deauthorize struct {
		KeyFingerprint string `cli:"-f, --fingerprint" env:"SFAB_AGENT_FINGERPRINT"`
	} `cli:"deauth, deauthz, deauthorize"`
}

func main() {
	opts.LogLevel = "info"

	opts.Hub.Bind = "127.0.0.1:4771"
	opts.Hub.Listen = "127.0.0.1:4772"
	opts.Hub.KeepAlive = 10

	opts.Agent.Hub = "127.0.0.1:4771"

	env.Override(&opts)
	log.SetupLogging(log.LogConfig{
		Type: "console",
		Level: opts.LogLevel,
	})

	command, args, err := cli.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! %s\n", err)
		os.Exit(1)
	}

	if command == "" || opts.Help {
		fmt.Printf("sfab - An example implementation of SSH-Fabric\n")
		fmt.Printf("\n")
		fmt.Printf("COMMANDS\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{hub}             Run an SFAB hub\n")
		fmt.Printf("\n")
		fmt.Printf("    -b, --bind    What IP:port to bind the SSH endpoint on. (@W{$SFAB_HUB_BIND})\n")
		fmt.Printf("    -l, --listen  What IP:port to bind the HTTP API on. (@W{$SFAB_HUB_LISTEN})\n")
		fmt.Printf("    -k, --key     Path to the private SSH host key. (@W{$SFAB_HUB_HOST_KEY})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{agent} NAME      Run an SFAB agent\n")
		fmt.Printf("\n")
		fmt.Printf("    -H, --hub     What IP:port of the hub to connect to. (@W{$SFAB_HUB})\n")
		fmt.Printf("    -k, --key     Path to the agent's private SSH key. (@W{$SFAB_AGENT_KEY})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{keys}            List known agents, their keys, and authorizations.\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{agents}          List authorized agents, by name.\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{responses}       Dump the responses from all agents.\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{auth} AGENT      Authorize an agent (by name and key)\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("    -f            SHA256 key fingerprint. (@W{$SFAB_AGENT_FINGERPRINT})\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{deauth} AGENT    Deauthorize an agent (by name and key)\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("    -f            SHA256 key fingerprint.\n")
		fmt.Printf("\n")
		fmt.Printf("  @C{ping} AGENT      Ping an agent, by identity.\n")
		fmt.Printf("                  Authorized agents should PONG us back.\n")
		fmt.Printf("                  Unauthorized agents should not.\n")
		fmt.Printf("\n")
		fmt.Printf("    -a, --api     The full URL of the hub HTTP API. (@W{$SFAB_HUB})\n")
		fmt.Printf("\n")
		os.Exit(0)
	}

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
		if opts.Hub.Listen == "" {
			fmt.Fprintf(os.Stderr, "Missing required --listen parameter (or SFAB_HUB_LISTEN environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "@M{sfab hub} starting  SSH server on @G{%s}\n", opts.Hub.Bind)
		fmt.Fprintf(os.Stderr, "         starting HTTP server on @G{%s}\n", opts.Hub.Listen)
		Hub()

	} else if command == "agent" {
		ok := true
		if opts.Agent.Hub == "" {
			fmt.Fprintf(os.Stderr, "Missing required --hub parameter (or SFAB_HUB environment variable)\n")
			ok = false
		}
		if opts.Agent.Key == "" {
			fmt.Fprintf(os.Stderr, "Missing required --key parameter (or SFAB_AGENT_KEY environment variable)\n")
			ok = false
		}
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Missing required agent name parameter\n")
			ok = false
		}
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Too many positional parameters!\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		key, err := sfab.ParseKeyFromFile(opts.Agent.Key)
		bail(err, "unable to load agent private key from %s", opts.Agent.Key)

		a := sfab.Agent{
			Identity:   args[0],
			Timeout:    30 * time.Second,
			PrivateKey: key,
		}

		fmt.Fprintf(os.Stderr, "@Y{sfab agent} connecting to hub at @M{%s}\n", opts.Agent.Hub)
		err = a.Connect("tcp4", opts.Agent.Hub, jsonnet)
		bail(err, "unable to connect to hub at '%s'", opts.Agent.Hub)
		os.Exit(0)

	} else if command == "keys" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		out, err := get("/keys")
		bail(err, "unable to list keys")
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else if command == "agents" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		out, err := get("/agents")
		bail(err, "unable to list agents")
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else if command == "responses" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		out, err := get("/responses")
		bail(err, "unable to list responses")
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else if command == "ping" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Missing required agent name parameter\n")
			ok = false
		}
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Too many positional parameters!\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}

		out, err := post("/ping", map[string]string{"Identity": args[0]})
		bail(err, "unable to ping %s", args[0])
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else if command == "auth" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if opts.Authorize.KeyFingerprint == "" {
			fmt.Fprintf(os.Stderr, "Missing required --fingerprint parameter (or SFAB_AGENT_FINGERPRINT environment variable)")
		}
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Missing required agent name parameter\n")
			ok = false
		}
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Too many positional parameters!\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		out, err := post("/authz", map[string]string{
			"Identity":    args[0],
			"Fingerprint": opts.Authorize.KeyFingerprint,
		})
		bail(err, "unable to authorize agent '%s' (%s)", args[0], opts.Authorize.KeyFingerprint)
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else if command == "deauth" {
		ok := true
		if opts.API == "" {
			fmt.Fprintf(os.Stderr, "Missing required --api parameter (or SFAB_HUB_API environment variable)\n")
			ok = false
		}
		if opts.Deauthorize.KeyFingerprint == "" {
			fmt.Fprintf(os.Stderr, "Missing required --fingerprint parameter (or SFAB_AGENT_FINGERPRINT environment variable)")
		}
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Missing required agent name parameter\n")
			ok = false
		}
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Too many positional parameters!\n")
			ok = false
		}
		if !ok {
			os.Exit(1)
		}
		out, err := post("/deauthz", map[string]string{
			"Identity":    args[0],
			"Fingerprint": opts.Deauthorize.KeyFingerprint,
		})
		bail(err, "unable to deauthorize agent '%s' (%s)", args[0], opts.Authorize.KeyFingerprint)
		fmt.Printf("%s\n", out)
		os.Exit(0)

	} else {
		fmt.Fprintf(os.Stderr, "Command not recognized\n")
		os.Exit(2)
	}

	os.Exit(0)
}
