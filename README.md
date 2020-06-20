sFAB - An SSH Fabric for Distributed Agent Orchestration
========================================================

[![Read our godoc!](https://godoc.org/github.com/jhunt/go-sfab?status.svg)](https://godoc.org/github.com/jhunt/go-sfab)

Welcome to sFAB, a utility library for stringing together
arbitrary agents, running across all types of IP networks, to form
an ad hoc execution fabric, all securely over SSH!

That's quite a mouthful.  We can do better.

sFAB lets you write programs that communicate securely, in a
hub-and-spoke topology.  The following concepts are key:

 - An **Agent** is a process running somewhere on the network, and
   performs tasks when asked to.

 - A **Hub** is a process running somewhere on the network.

 - Each **Agent** connects to one or more **Hubs**.

 - Commands are sent by consumers, via a **Hub**, and bound for a
   specific **Agent**.


How to Run a Hub
----------------

Before you can get a lot of mileage out of sFAB, we're going to need to spin
up a Hub:

```go
import (
  "github.com/jhunt/go-sfab"
)

func main() {
  hub := &Hub{
    Bind:        "*:4000",
    HostKeyFile: "host_key",
  }

  hub.AuthorizeKeys("authorized_keys")
  hub.ListenAndServe()
}
```

This will spin up a Hub (an SSH server) bound to all interfaces on TCP port
4000, using the contents of the `host_key` file as its private key, and
allowing anyone with a username + keypair listed in the `authorized_keys`
file to connect (as an Agent).


How Agents Connect to the Hub
-----------------------------

Agents connect to a Hub via the `Connect()` method:

```go
import (
  "io"

  "github.com/jhunt/go-sfab"
)

func main() {
  agent := &sfab.Agent{
    Identity:       "bob@postgres.ql",
    PrivateKeyFile: "id_rsa",
  }

  handler := func(msg []byte, stdout, stderr io.Writer) (int, error) {

    // ... do something useful here ...

    return 0, nil
  }

  agent.Connect("tcp4", "hub.fqdn:4000", handler)
}
```

This Agent's `Connect()` method will present the `Identity` of
"bob@postgres.ql" to the remote Hub, along with the private key stored in
the `id_rsa` file.

![Agent / Hub Communication](docs/agent-to-hub.png)

The authentication handshake (part of the [SSH-USERAUTH][1] protocol,
handled by lower layers of this library) identifies the agent.  The Hub must
have pre-authorized the use of our private key (from `id_rsa`) as the
identity "bob@postgres.ql".

The Hub records this information, and associates the agent name with the
SSH connection.  The next time one of its callers asks for something to be
executed against an agent named "bob@postgres.ql", the Hub will relay that
to this Agent, by doing the following:

  1. Opening an SSH "session" channel (per [section 5.1 of RFC-4254][2])
  2. Issuing an "exec" request (per [section 6.5 of RFC-4254][3])
  3. Copying output from the remote agent back to the caller, by way of the
     _responses_ (Go) channel.


Running Something on the Fabric
-------------------------------

To actually use this fabric, we need a third party, our _client_.
A client is just the Go code that calls into the Hub and asks for
things to be run against remote agents:

![Agent Command Execution](docs/agent-exec.png)

Here's an adjusted Hub implementation that attempts to
re-reconcile bob every 30 seconds:

```go
import (
  "fmt"
  "time"
  "os"

  "github.com/jhunt/go-sfab"
)

func main() {
  hub := &sfab.Hub{
    Bind:        "*:4000",
    HostKeyFile: "host_key",
  }

  hub.AuthorizeKeys("authorized_keys")

  go func() {
    t := time.NewTicker(30 * time.Second)
    for range t.C {

      /* every 30 seconds: */
      fmt.Fprintf(os.Stderr, "running reconciliation...\n")
      reply, _ := hub.Send("bob@postgres.ql", []byte("reconcile(x)"))

      for msg := range reply {
        if msg.IsStdout() || msg.IsStderr() {
          fmt.Printf("%s\n", msg.Text())

        } else if msg.IsError() {
          fmt.Fprintf(os.Stderr, "oops: %s\n", msg.Error())

        } else if msg.IsExit() {
          fmt.Fprintf(os.Stderr, "command exited %d\n", msg.ExitCode())
        }
      }
    }
  }()
  hub.ListenAndServe()
}
```

Now, our Hub will boot up, listen on `*:4000` as before (same host
keys, same authorizations), and then every 30 seconds will tell
bob to reconcile.

Next, let's modify the Agent implementation to do something:

```go
import (
  "io"
  "fmt"

  "github.com/jhunt/go-sfab"
)

func main() {
  agent := &sfab.Agent{
    Identity:       "bob@postgres.ql",
    PrivateKeyFile: "id_rsa",
  }

  handler := func(msg []byte, stdout, stderr io.Writer) (int, error) {

    if string(msg) == "reconcile(x)" {
      fmt.Fprintf(stdout, "BEGIN RECONCILIATION\n")
      reconciliation()
      fmt.Fprintf(stdout, "END RECONCILIATION\n")
      return 0, nil
    }

    fmt.Fprintf(stderr, "I'm sorry; I don't know how to '%s'\n", string(msg))
    return 1, nil
  }

  agent.Connect("tcp4", "hub.fqdn:4000", handler)
}
```


When Agents Aren't Available
----------------------------

So what happens if an agent is unavailable?

If it's a temporary availability issue, and the Agent is
maintaining its connection to the Hub, the messages sent by
`Send()` will be queued in the goroutine.

If the Agent has left (or has not yet connected), the Hub will
respond to the caller accordingly:

![No Such Agent](docs/no-such-agent.png)


Halting an Agent
----------------

At times, you may want an _in-line_ method of terminating an
Agent on behalf of a Client, through the Hub.  The handler
function that gets passed to the Agent's `Connect()` method
can return an error to signal this condition.  The Agent will
process the error (logging it, mostly), and then break out of its
main event loop, effectively shutting down.

Note that this error will not propagate to the caller of
`Connect()`; the error returned by that function is intended
solely to signal a _problem_ handling the connection, and exiting
early is not problematic.

Here's an example:

```go
handler := func(msg []byte, stdout, stderr io.Writer) (int, error) {

  if string(msg) == "EXEUNT" {
    fmt.Fprintf(stdout, "exiting...\n")
    return 0, fmt.Errorf("exit requested")
  }

  // ... other, more useful work here ...

  return 0, nil
}

agent.Connect("tcp4", "hub.fqdn:4000", handler)
```


The Example SFAB Ping System
============================

This repository ships with a fully-functional, if a touch silly
example implementation.  To build it:

    $ go build ./example/demo

That will put a multi-call binary, named `sfab` in your current
working directory (the root of the codebase).  The example system
consists of a **hub**, which binds a loopback SSH server and an
HTTP control module (also on loopback), an **agent** that connects
to the SSH port and waits for PINGs to PONG back, and a handful of
CLI sub-commands to interrogate the internal state of the hub and
to poke the agents.

![Screenshot of Example SFAB Ping System](docs/example.png)

It also ships with an `envrc` that sets all the environment
variables to make running a single hub + agent trivial:

    $ source example/demo/envrc

To play along at home, run the following commands, in different
terminal windows or tmux panes:

    $ source example/demo/envrc && ./demo hub
    $ source example/demo/envrc && ./demo agent test-agent
    $ source example/demo/envrc && watch ./demo keys

Then, you can ping the **test-agent** agent:

    $ source example/demo/envrc
    $ ./demo ping test-agent

This should fail, since we haven't authorized the test-agent's
key; we don't yet trust that agent.  To establish that trust:

    $ source example/demo/envrc  # if you hven't already...
    $ ./demo authz test-agent

You'll notice that the `watch` command we ran will list the key as
authorized now, and the agent as "known".  Test it out by pinging
the agent again:

    $ ./demo ping test-agent

This time, you should see some output from the hub, detailing the
PONG response from the agent, and you should see some chatter on
the agent pane / terminal about the receipt of a PING.

You can deauthorize the test-agent by running:

    $ ./demo deauthz test-agent

You can also get a list of all the PING / PONG exchanges by
running:

    $ ./demo responses

To play with it some more, refer to the built-in help:

    $ ./demo --help
To play with it some more, refer to the built-in help:

   $ ./demo --help
    demo - An example implementation of SSH-Fabric

    COMMANDS

      hub             Run an SFAB hub

        -b, --bind    What IP:port to bind the SSH endpoint on. ($SFAB_HUB_BIND)
        -l, --listen  What IP:port to bind the HTTP API on. ($SFAB_HUB_LISTEN)
        -k, --key     Path to the private SSH host key. ($SFAB_HUB_HOST_KEY)

      agent NAME      Run an SFAB agent

        -H, --hub     What IP:port of the hub to connect to. ($SFAB_HUB)
        -k, --key     Path to the agent's private SSH key. ($SFAB_AGENT_KEY)

      keys            List known agents, their keys, and authorizations.

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)

      agents          List authorized agents, by name.

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)

      responses       Dump the responses from all agents.

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)

      auth AGENT      Authorize an agent (by name and key)

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)
        -f            SHA256 key fingerprint. ($SFAB_AGENT_FINGERPRINT)

      deauth AGENT    Deauthorize an agent (by name and key)

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)
        -f            SHA256 key fingerprint.

      ping AGENT      Ping an agent, by identity.
                      Authorized agents should PONG us back.
                      Unauthorized agents should not.

        -a, --api     The full URL of the hub HTTP API. ($SFAB_HUB)

Go ahead!  Try spinning up some other agents, either reusing the
existing `example/demo/agent_key`, or generating a new one.


Generating SFAB RSA Keys
========================

The state of the art in generating RSA private and public keys is
a bit saddening, to say the least.  To make life easier for
ourselves, and others, we wrote a small helper utility named
`sfab`, and packaged it up in this repository.

You can use it to generate a keypair like this:

    $ sfab keygen
    generating a 2048-bit rsa private key to standard output...
    -----BEGIN RSA PRIVATE KEY-----
    MIIEowIBAAKCAQEA2jRmGj8AdGB+QhwPcXPltij33NnP/Q5tyYaE+n7WPsrnWQzh
    8IeCVX2Jkxnjn7uT2y4kNDEbY2P8DewWMlgTCyulIlZgAFxNGnrvRECkH6wVRlWM
    nlx3UlQgNExIIfDg0Z8A7DOChFD1GGe/twN37bgtempHUT2TDPGjK0jpMmcGbI9q
    rvnidoeipXifgpSROOxs8qnWA8BVgdqUqGiA5FWRV2yTXO9G07jsP4Vf1Vdu87Eb
    XMlRxAcFQOtf+TjbPGtSkDeyJSkjsHmT/+JYvJQlKL4gg8K0+Uw2/aemLetvwsCa
    Po9RO5XguSDGC8ecqE+758Rv6Jiz26UFcN21mwIDAQABAoIBADU1Y9Rwmn7GbONY
    Dlf/B/vkf5SGNJajpxfmSpT+yuWi+skXQB00Hg8UvrB1NrFjv0lKNh/X/bK91d/b
    OwppubMWoH0ADQbpew/C/IdrL9pdTw7Im7KeH6aexv+ZvlkwcNIM8N+oK+JyD2I1
    ImXRXbRFHinksqFX33zfi4dpjxvv2eSImeNRI4mY4e9zNCYzMgH/51N9MVQNwEFu
    E2ExJhyPnGa2Pld1FewYczvhIkPPhB6UZsw0M9FicrZfFN5GJwVr+P121S/FMDIm
    N0j2d7utkqfh639sOM7Y4na5T8ztn/UuC0xHCnI2eDpXUhT+47oERYqQYifyz1by
    nRzIXZkCgYEA20Bx474++BJ6jmNyLzxnxMLDaFv1oAOaN8JrVhzQ++sDQ92M+UKX
    BL4WqCw4LYwE+UIcsRgAw/GXTuqLB7L4uAfujyx/uPR1o4v9qE9Bbx23dgKyE8fa
    JJzAEw8AzhQ2Rxnlof28EqMySKZ5hDuG+/mF8czzV7Bg6ASpCmszpkUCgYEA/scH
    A31UI86mpRzTpg89gNENt17mwP5isfgKXAxL1zhgdszLKSxwfSTMQzSAypieqdFq
    zsMoULtpq+lzRoUHXIMLdD+hcLLRVBnPDWxT0WGmAYLaIN1L7RxMz+sx+hrexy/d
    F7WdS//G8/YEwNr0IBFKC3sj7vceAEIGLweQGl8CgYANjW88x5620NWYhZciSlaT
    uAJCKiUFrhNTTQY7fW6Bk+ZQxJoZMj+JgIKABLO4RQZtreN+IuCxO/bBXwgXAr4T
    ChJ58SM3+Ixo/pdAVx3RU4HqnoIJbPoT90h8JRpEosU0Ieqq17dPY50RFpE+Vjui
    yaq45pVIH1PuI5ObXEMGDQKBgH+H1r3rbUOjPI1nuZXlCQ9DFc+VJIK0k2isVVsw
    GIJ5yOeOqZ9JjGFIGysEy/cdoEY2+nTXAYYv2bxg1gAVtNKdEg21kDUefWPQq3D3
    qZkkfPlCq7dJrqjYwqZM/65G5v7juZMVqNATD16s5kI3F345yUun7xu9/Pl6EyhV
    a8+XAoGBAJmeUEDVGv9uiICM8abfl7wyxekD3pdfajov5gWj5PKYW7tLBRfbbmcy
    UdM0PnqUgKgICqhUfMcRlc1+anH8JPYQnfDgTzimKei4Y07KJgeyrZyi0nojk441
    UGr9V8J7AypffH88uQt3msGhkP1wPA60PIhoIkzAYjPRrSda3PU7
    -----END RSA PRIVATE KEY-----

The `sfab key` command lets you validate and optionally print the
private and public key components of a given key file:

    $ sfab keygen > test.pem
    $ sfab key -q --private --public test.pem
    $ echo $?
    0
    $ sfab key --private --public test.pem
    -----BEGIN RSA PRIVATE KEY-----
    MIIEpAIBAAKCAQEAtgQJei7jzcr4HTNydR6GOVdFr/c2zdnhCku3BHLogliV1hNA
    9oUqajCNUvpkDRMxSWCIdiQdhms5N/o+nBJQgEj259eiKdcsrBKGk7SjDXgdjBTD
    yTgx550/3O2A0qACk14Xf2xIKSidOByIBFFjbh5oQq0nTZnq8Sa5lZ6lx4kzvHy3
    Qhg+mbv5ocdrgpNf6Wi1aZTzm39bvwfh5UvZxXwgBP4s4Nf8LAbatpgy8S4jt5kg
    PsCrD0Nw1tQTrgtgrQO3KjPwizdnUwkSS3jAX40hKW7d229IS/0hrHBn4Tc0HaWA
    F2fncwYD8BwHFI1ZmwMMWHXzdNFvDC/CInALgQIDAQABAoIBAQCI7SSMDDZHnIdm
    wEs8nEfxXxwHsnxCA8Rt0GPQLvmHBUlDEJzwV2F/HSNT5xXZWDTdtIc0CRtFAei7
    mb/7/TMeLsW/x9Iwk7UBhbZD68I14cSomv5mo6W3Te67LtiYmtMVNdWLgFQQ0oLj
    sWMJjIi2UJ2yezxz/nRmEfHPAthH8EQCoxR1BROdlaaeyfz6WVfFp27NzO3Uq9ZZ
    8DJUPuzmHiJmQcopgtYknzBXoPC7TgKBL1ozMohbhQo7wJQTZsrxjMeBuVDSknZf
    uXbrf1x+SfcME4fENxI6NdNQL23LFHuE9LMuapIURPFpDVDJVGs9Van2g5KiVYtt
    tHl/doqBAoGBAMycXLDWulilAHKkDNHAarTk8cgL7F3kweyhPF2gIt2gudTKW5WL
    iUuPdgbDDvdkcx2wbKYwpzWh0EHCGoNL770RT7gOMVf9EcjzGZPfid7tV7lTKrSF
    rT6PNIM3AHVuOoiNe/Su/8jf/vhAx1xYzD4eAb1IprpiDNU/ww17PlipAoGBAOO6
    6FB8CX8MP3ejzWC7mAku5tVUIGNGrWztI3c0HKGaGpLnc1OzFBmDVW5nHTodhj+D
    VlTXxZeKunJnEtA4QyxDJuFH+dFZ7LQ+mcCMUojv0qiI/8GnGV+VV1zookslA+J2
    BayVAEcKSyw4lc1lJdQKfD1KycPOgbEN7FSs7SsZAoGBAJc5NZSJj352rQcN0l+w
    G+2IjF4y2cZnjSBL5qa39SziXpYz3U6Aq7+Jz0yeG9CikCYwP4zwxpRv/NCtGhVS
    KWUOh2HcM4H5i4x+h7fv7iEyP7S8EAMODYYAJ2qXWyJvjplgIG4ubOPVEcf/cJkD
    FCkQ2m5AZOG6GAwALTQok2LJAoGAeJZNU+kKYkMLDIApTZKoZBx40Pe2GrufM7Gy
    ePmYO55NegKjF+wYtZXM1CQ33sEpiFI0kcKb963XIqMf3DAeqb55RQuG5mt2os8P
    PV1QiwBrDSFWRKYGEo4S77ijz6SvFOioAoX32qZBvg3MjTGFQW238gC7LDsag7W6
    se/LumkCgYBt0+ylBg9zh0QIKFXvBecRVtIgvjV57mWvFQkB/4/0F3P2Hz87Jja5
    GLr9neUNVcdD7RRL10/m/V0uJXVrBO7GinhsEFfuzNdMBM+7USdnOv2r1rQwJfsO
    OlNmor/8YUVY8/YIDU6cZ7dHfxQBWjoKj8dt8jMMJGxqsJ96HsgtFw==
    -----END RSA PRIVATE KEY-----
    -----BEGIN RSA PUBLIC KEY-----
    MIIBCgKCAQEAtgQJei7jzcr4HTNydR6GOVdFr/c2zdnhCku3BHLogliV1hNA9oUq
    ajCNUvpkDRMxSWCIdiQdhms5N/o+nBJQgEj259eiKdcsrBKGk7SjDXgdjBTDyTgx
    550/3O2A0qACk14Xf2xIKSidOByIBFFjbh5oQq0nTZnq8Sa5lZ6lx4kzvHy3Qhg+
    mbv5ocdrgpNf6Wi1aZTzm39bvwfh5UvZxXwgBP4s4Nf8LAbatpgy8S4jt5kgPsCr
    D0Nw1tQTrgtgrQO3KjPwizdnUwkSS3jAX40hKW7d229IS/0hrHBn4Tc0HaWAF2fn
    cwYD8BwHFI1ZmwMMWHXzdNFvDC/CInALgQIDAQAB
    -----END RSA PUBLIC KEY-----

With `--quiet` (`-q`) mode, you can even validate a bunch of keys
in a directory:

    $ sfab key -q ~/my/keys/*.pem

... or via a find + xargs shell pipeline:

    $ find / -name 'rsa*.pem' | xargs sfab key -q


Contributing
============

This code is licensed MIT.  Enjoy.

If you find a bug, please raise a [Github Issue][issues] first,
before submitting a PR.

Happy Hacking!



[1]: https://tools.ietf.org/html/rfc4252
[2]: https://tools.ietf.org/html/rfc4254#section-5.1
[3]: https://tools.ietf.org/html/rfc4254#section-6.5

[issues]: https://github.com/jhunt/go-sfab/issues
