package sfab_test

import (
	"fmt"
	"io"
	"syscall"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jhunt/go-sfab"
)

func TestAllTheThings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSH Fabric Test Suite")
}

var _ = Describe("end-to-end", func() {
	port := 5770
	slack := func(cmd []byte, _, _ io.Writer) (int, error) {
		return 0, nil
	}

	Context("a hub", func() {
		var (
			agent *sfab.Agent
			hub   *sfab.Hub
		)

		BeforeEach(func() {
			port++

			ak, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			agent = &sfab.Agent{
				Identity:   fmt.Sprintf("agent@test-%d", port),
				PrivateKey: ak,
				Timeout:    30 * time.Second,
			}
			agent.AcceptAnyHostKey()

			hk, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			hub = &sfab.Hub{
				Bind:      fmt.Sprintf("127.0.0.1:%d", port),
				HostKey:   hk,
				KeepAlive: 10 * time.Second,
			}
			hub.AuthorizeKey(agent.Identity, ak.PublicKey())
		})

		It("should not know about an agent before connect()", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())
			Ω(hub.Agents()).Should(Equal([]string{}))
		})

		It("should allow authorized agents to connect()", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())

			go agent.Connect("tcp4", hub.Bind, slack)
			<-hub.Await(agent.Identity)
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeTrue())
			Ω(hub.Agents()).Should(Equal([]string{agent.Identity}))
		})

		It("should not allow unauthorized agents to connect()", func() {
			rogueKey, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			rogue := &sfab.Agent{
				Identity:   fmt.Sprintf("rogue@test-%d", port),
				PrivateKey: rogueKey,
				Timeout:    30 * time.Second,
			}
			rogue.AcceptAnyHostKey()

			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(rogue.Identity)).Should(BeFalse())

			Ω(rogue.Connect("tcp4", hub.Bind, slack)).ShouldNot(Succeed())
			Ω(hub.KnowsAgent(rogue.Identity)).Should(BeFalse())
		})

		It("should not allow deauthorized agents to connect()", func() {
			hub.DeauthorizeKey(agent.Identity, agent.PrivateKey.PublicKey())
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())

			Ω(agent.Connect("tcp4", hub.Bind, slack)).ShouldNot(Succeed())
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())
		})

		It("should not allow deauthorized agents to connect()", func() {
			rogueKey, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			hub.DeauthorizeKey(agent.Identity, rogueKey.PublicKey())
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())

			agent.PrivateKey = rogueKey
			Ω(agent.Connect("tcp4", hub.Bind, slack)).ShouldNot(Succeed())
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())
		})

		It("should not allow name/key mismatch during connect()", func() {
			agent.Identity = fmt.Sprintf("mismatch@test-%d", port)
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())

			Ω(agent.Connect("tcp4", hub.Bind, slack)).ShouldNot(Succeed())
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())
		})

		It("should not allow multiple agent registrations", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeFalse())

			ch := make(chan string)
			go agent.Connect("tcp4", hub.Bind, func(_ []byte, _, _ io.Writer) (int, error) {
				ch <- "from agent"
				return 0, nil
			})
			<-hub.Await(agent.Identity)
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeTrue())
			Ω(hub.Agents()).Should(Equal([]string{agent.Identity}))

			clone := &sfab.Agent{
				Identity:   agent.Identity,
				PrivateKey: agent.PrivateKey,
				Timeout:    agent.Timeout,
			}
			clone.AcceptAnyHostKey()

			clone.Connect("tcp4", hub.Bind, func(_ []byte, _, _ io.Writer) (int, error) {
				ch <- "from clone"
				return 0, nil
			}) // should return immediately
			Ω(hub.KnowsAgent(agent.Identity)).Should(BeTrue())
			Ω(hub.Agents()).Should(Equal([]string{agent.Identity}))

			r, err := hub.Send(agent.Identity, []byte{}, 5*time.Second)
			Ω(err).ShouldNot(HaveOccurred())
			go hub.IgnoreReplies(r)

			Ω(<-ch).Should(Equal("from agent"))
		})
	})

	Context("a 1:1 hub:agent topology", func() {
		var (
			agent *sfab.Agent
			hub   *sfab.Hub
		)

		BeforeEach(func() {
			port++

			ak, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			agent = &sfab.Agent{
				Identity:   fmt.Sprintf("agent@test-%d", port),
				PrivateKey: ak,
				Timeout:    30 * time.Second,
			}
			agent.AcceptAnyHostKey()

			hk, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			hub = &sfab.Hub{
				Bind:      fmt.Sprintf("127.0.0.1:%d", port),
				HostKey:   hk,
				KeepAlive: 10 * time.Second,
			}
			hub.AuthorizeKey(agent.Identity, ak.PublicKey())
		})

		It("should allow a hub → agent dispatch", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			go agent.Connect("tcp4", hub.Bind, slack)
			<-hub.Await(agent.Identity)

			res, err := hub.Send(agent.Identity, []byte("hi"), 5*time.Second)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res).ShouldNot(BeNil())

			r := <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsExit()).Should(BeTrue())
		})

		It("should return agent stdout to hub customer", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			go agent.Connect("tcp4", hub.Bind, func(_ []byte, out, _ io.Writer) (int, error) {
				fmt.Fprintf(out, "this is a TEST message")
				return 0, nil
			})
			<-hub.Await(agent.Identity)

			res, err := hub.Send(agent.Identity, []byte("hi"), 5*time.Second)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res).ShouldNot(BeNil())

			r := <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsStdout()).Should(BeTrue())
			Ω(r.Text()).Should(Equal("this is a TEST message"))

			r = <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsExit()).Should(BeTrue())
		})

		It("should return agent stderr to hub customer", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			go agent.Connect("tcp4", hub.Bind, func(_ []byte, _, oops io.Writer) (int, error) {
				fmt.Fprintf(oops, ":sad trombone:")
				return 0, nil
			})
			<-hub.Await(agent.Identity)

			res, err := hub.Send(agent.Identity, []byte("hi"), 5*time.Second)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res).ShouldNot(BeNil())

			r := <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsStderr()).Should(BeTrue())
			Ω(r.Text()).Should(Equal(":sad trombone:"))

			r = <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsExit()).Should(BeTrue())
		})

		It("should handle multiline agent output", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			go agent.Connect("tcp4", hub.Bind, func(_ []byte, out, _ io.Writer) (int, error) {
				fmt.Fprintf(out, "this\nwas all printed\ntogether\n")
				return 0, nil
			})
			<-hub.Await(agent.Identity)

			res, err := hub.Send(agent.Identity, []byte("hi"), 5*time.Second)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res).ShouldNot(BeNil())

			r := <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsStdout()).Should(BeTrue())
			Ω(r.Text()).Should(Equal("this"))

			r = <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsStdout()).Should(BeTrue())
			Ω(r.Text()).Should(Equal("was all printed"))

			r = <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsStdout()).Should(BeTrue())
			Ω(r.Text()).Should(Equal("together"))

			r = <-res
			Ω(r).ShouldNot(BeNil())
			Ω(r.IsExit()).Should(BeTrue())
		})
	})

	Context("a 1:n hub:agent topology", func() {
		var (
			agents []*sfab.Agent
			hub    *sfab.Hub
		)

		BeforeEach(func() {
			port++

			hk, err := sfab.GeneratePrivateKey(1024)
			Ω(err).ShouldNot(HaveOccurred())

			hub = &sfab.Hub{
				Bind:      fmt.Sprintf("127.0.0.1:%d", port),
				HostKey:   hk,
				KeepAlive: 10 * time.Second,
			}

			agents := make([]*sfab.Agent, 5)
			for i := range agents {
				ak, err := sfab.GeneratePrivateKey(1024)
				Ω(err).ShouldNot(HaveOccurred())

				agent := &sfab.Agent{
					Identity:   fmt.Sprintf("agent/%d@test-%d", i, port),
					PrivateKey: ak,
					Timeout:    30 * time.Second,
				}
				agent.AcceptAnyHostKey()

				hub.AuthorizeKey(agent.Identity, ak.PublicKey())
				agents[i] = agent
			}
		})

		It("should allow registration of multiple agents", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			for _, agent := range agents {
				go agent.Connect("tcp4", hub.Bind, slack)
				<-hub.Await(agent.Identity)
			}

			for _, agent := range agents {
				Ω(hub.KnowsAgent(agent.Identity)).Should(BeTrue())
			}
		})

		It("should distribute work to all named agents", func() {
			Ω(hub.Listen()).Should(Succeed())
			go hub.Serve()

			ch := make(chan int)
			for _, agent := range agents {
				go agent.Connect("tcp4", hub.Bind, func(_ []byte, _, _ io.Writer) (int, error) {
					ch <- 1
					return 0, nil
				})
				<-hub.Await(agent.Identity)
			}

			for _, agent := range agents {
				r, err := hub.Send(agent.Identity, []byte("count!"), 5*time.Second)
				Ω(err).ShouldNot(HaveOccurred())
				go hub.IgnoreReplies(r)
			}

			for range agents {
				Ω(<-ch).Should(Equal(1))
			}
		})
	})

	Context("large network of agents", func() {
		scaleTo := func(n int) func() {
			return func() {
				Ω(n).Should(BeNumerically(">", 0))

				var rlim syscall.Rlimit
				err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
				if err != nil {
					Skip("unable to get rlimits for process; cannot determine if we will have enough open files...")
				}
				rlim.Cur = rlim.Max
				err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim)
				if err != nil {
					Skip("unable to max out fd rlimits for process; unsure if we will have enough open files...")
				}
				err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
				if err != nil {
					Skip("unable to get rlimits for process; cannot determine if we will have enough open files...")
				}
				if rlim.Cur < uint64(n)*2 {
					Skip(fmt.Sprintf("current fd rlimit is %d; not enough to spin a hub and %d SSH clients", rlim.Cur, n))
				}

				// all good, carry on...
				port++

				hk, err := sfab.GeneratePrivateKey(1024)
				Ω(err).ShouldNot(HaveOccurred())

				hub := &sfab.Hub{
					Bind:      fmt.Sprintf("127.0.0.1:%d", port),
					HostKey:   hk,
					KeepAlive: 10 * time.Second,
				}
				Ω(hub.Listen()).Should(Succeed())
				go hub.Serve()

				ch := make(chan int, n)
				for i := 0; i < n; i++ {
					ak, err := sfab.GeneratePrivateKey(1024)
					Ω(err).ShouldNot(HaveOccurred())

					agent := &sfab.Agent{
						Identity:   fmt.Sprintf("agent/%d@test-%d", i, port),
						PrivateKey: ak,
						Timeout:    30 * time.Second,
					}
					agent.AcceptAnyHostKey()
					hub.AuthorizeKey(agent.Identity, ak.PublicKey())
					go agent.Connect("tcp4", hub.Bind, func(_ []byte, _, _ io.Writer) (int, error) {
						ch <- 1
						return 0, nil
					})
				}

				for i := 0; i < n; i++ {
					<-hub.Await(fmt.Sprintf("agent/%d@test-%d", i, port))
				}
				for i := 0; i < n; i++ {
					r, err := hub.Send(fmt.Sprintf("agent/%d@test-%d", i, port), []byte{}, 5*time.Second)
					Ω(err).ShouldNot(HaveOccurred())
					go hub.IgnoreReplies(r)
				}
				for i := 0; i < n; i++ {
					Ω(<-ch).Should(Equal(1))
				}
			}
		}

		It("should scale to 10 agents", scaleTo(10))
		It("should scale to 25 agents", scaleTo(25))
		It("should scale to 50 agents", scaleTo(50))
		It("should scale to 100 agents", scaleTo(100))
		It("should scale to 200 agents", scaleTo(200))
		It("should scale to 400 agents", scaleTo(400))
	})
})
