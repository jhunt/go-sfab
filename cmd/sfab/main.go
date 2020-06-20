package main

import (
	"io/ioutil"
	"os"
	"strings"

	fmt "github.com/jhunt/go-ansi"
	"github.com/jhunt/go-cli"
	env "github.com/jhunt/go-envirotron"
	"github.com/jhunt/go-log"
	"github.com/mattn/go-isatty"

	"github.com/jhunt/go-sfab"
)

var opts struct {
	LogLevel string `cli:"-L, --log-level" env:"SFAB_LOG_LEVEL"`
	Help     bool   `cli:"-h, --help"`

	Keygen struct {
		Bits int `cli:"-b, --bits"`
	} `cli:"keygen"`

	Key struct {
		Quiet   bool `cli:"-q, --quiet, --no-quiet" env:"SFAB_QUIET"`
		Private bool `cli:"-k, --private, --priv, --no-private, --no-priv"`
		Public  bool `cli:"-p, --public, --pub, --no-public, --no-pub"`
	} `cli:"key"`
}

func main() {
	opts.LogLevel = "info"
	opts.Keygen.Bits = 2048

	env.Override(&opts)
	log.SetupLogging(log.LogConfig{
		Type:  "console",
		Level: opts.LogLevel,
	})

	command, args, err := cli.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! %s\n", err)
		os.Exit(1)
	}

	if opts.Help || (command == "" && len(args) == 0) {
		fmt.Printf("@*{sfab} - A helper utility for the @*{SSH Fabric} framework\n")
		fmt.Printf("\n")
		fmt.Printf("@W{COMMANDS}\n")
		fmt.Printf("\n")
		fmt.Printf("  @G{keygen}            Generate SFAB RSA private keys\n")
		fmt.Printf("\n")
		fmt.Printf("    -b, --bits N    How strong to make the RSA key, must be one of\n")
		fmt.Printf("                    1024, 2048, or 4096.\n")
		fmt.Printf("\n")
		fmt.Printf("  @G{key} @C{PATH}          Inspect, validate, and dump SFAB RSA keys\n")
		fmt.Printf("\n")
		fmt.Printf("    -q, --quiet     Print nothing.\n")
		fmt.Printf("    --private       Validate that the private key is present.\n")
		fmt.Printf("                    (Prints the PEM-encoded key if ! --quiet)\n")
		fmt.Printf("    --public        Validate that the public key is present.\n")
		fmt.Printf("                    (Prints the public key if ! --quiet)\n")
		fmt.Printf("\n")
		os.Exit(0)
	}

	if command == "keygen" {
		switch opts.Keygen.Bits {
		case 1024, 2048, 4096:
			if isatty.IsTerminal(1) {
				fmt.Fprintf(os.Stderr, "generating a %d-bit rsa private key to standard output...\n", opts.Keygen.Bits)
			}

		default:
			fmt.Fprintf(os.Stderr, "unable to generate a %d-bit rsa private key;\n")
			fmt.Fprintf(os.Stderr, "please pick either 1024, 2048, or 4096 for --bits\n")
			os.Exit(1)
		}

		k, err := sfab.GenerateKey(opts.Keygen.Bits)
		if err != nil {
			fmt.Fprintf(os.Stderr, "keygen failed: %s\n", err)
			os.Exit(2)
		}
		fmt.Printf("%s", k.EncodeString())
		os.Exit(0)

	} else if command == "key" {
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "USAGE: sfab key [options] @Y{PATH}\n")
			fmt.Fprintf(os.Stderr, "missing required @Y{PATH} argument!\n")
			os.Exit(1)
		}

		errors := false
		failures := false
		for _, path := range args {
			var in *os.File
			if path == "-" {
				in = os.Stdin
			} else {
				f, err := os.Open(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: @R{%s}\n", path, err)
					failures = true
					continue
				}
				in = f
			}

			b, err := ioutil.ReadAll(in)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: read failed: @R{%s}\n", path, err)
				failures = true
				continue
			}

			k, err := sfab.ParseKey(b)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: parse failed: @R{%s}\n", path, err)
				failures = true
				continue
			}

			if opts.Key.Private {
				if !k.IsPrivateKey() {
					fmt.Fprintf(os.Stderr, "%s does not contain a private key\n", path)
					errors = true
				}
				if !opts.Key.Quiet {
					fmt.Printf("%s", k.Private().EncodeString())
				}
			}

			if opts.Key.Public {
				if !k.IsPublicKey() {
					fmt.Fprintf(os.Stderr, "%s does not contain a public key\n", path)
					errors = true
				}
				if !opts.Key.Quiet {
					fmt.Printf("%s", k.Public().EncodeString())
				}
			}

			if !opts.Key.Private && !opts.Key.Public {
				if !opts.Key.Quiet {
					fmt.Printf("%s", k.EncodeString())
				}
			}

			if in != os.Stdin {
				in.Close()
			}
		}

		if errors {
			os.Exit(2)
		} else if failures {
			os.Exit(3)
		}
		os.Exit(0)

	} else {
		if command == "" {
			fmt.Fprintf(os.Stderr, "command `%s' not recognized\n", strings.Join(args, " "))
		} else {
			fmt.Fprintf(os.Stderr, "command `%s' not recognized\n", command)
		}
		os.Exit(2)
	}

	os.Exit(0)
}
