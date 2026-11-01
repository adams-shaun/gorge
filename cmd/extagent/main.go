// Command extagent runs gorge's reference gorge-agent/1 agent as a
// process: over stdin/stdout by default, or on a Unix socket with
// -listen. It is the binary shape an external training system's adapter
// mirrors; the agent itself is refagent.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses args and serves the reference agent, returning the process
// exit code. It is split out of main so tests can drive it in-process.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("extagent", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policy := fs.String("policy", "random", "answer policy: random|first")
	agent := fs.String("agent", "", "agent name in the hello (default: the policy name)")
	modelVersion := fs.String("model-version", "", "model_version in the hello (default: 1)")
	listen := fs.String("listen", "", "serve on unix:<path> instead of stdin/stdout")
	skipForced := fs.Bool("skip-forced", false, "declare forced decisions are answered by gorge")
	paymentActions := fs.Bool("payment-actions", false, "declare payment_actions support")
	targetHints := fs.Bool("target-hints", false, "declare target_hints support")
	viewOmit := fs.String("view-omit", "all", "declared view omissions: all|none|comma-separated names")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *policy != "random" && *policy != "first" {
		fmt.Fprintf(stderr, "extagent: unknown -policy %q (want random|first)\n", *policy)
		return 2
	}
	omit, err := parseViewOmitFlag(*viewOmit)
	if err != nil {
		fmt.Fprintf(stderr, "extagent: %v\n", err)
		return 2
	}
	cfg := refagent.Config{
		Agent:          *agent,
		ModelVersion:   *modelVersion,
		Policy:         *policy,
		MaxBatch:       0,
		SkipForced:     *skipForced,
		PaymentActions: *paymentActions,
		TargetHints:    *targetHints,
		ViewOmit:       omit,
	}
	if *listen == "" {
		if err := refagent.Serve(stdin, stdout, cfg); err != nil {
			fmt.Fprintf(stderr, "extagent: %v\n", err)
			return 1
		}
		return 0
	}
	return runListen(*listen, cfg, stderr)
}

// parseViewOmitFlag turns -view-omit into hello names: "all" is every name
// in wire.ViewOmitNames — the reference policies read only the decision, and
// an ext:exec: spec cannot pass flags, so the binary's defaults are what a
// spec gets — and "none" declares no omission.
func parseViewOmitFlag(v string) ([]string, error) {
	switch v {
	case "all":
		names := make([]string, 0, len(wire.ViewOmitNames))
		for _, e := range wire.ViewOmitNames {
			names = append(names, e.Name)
		}
		return names, nil
	case "none":
		return nil, nil
	}
	names := strings.Split(v, ",")
	for _, n := range names {
		if _, err := wire.ParseViewOmit([]string{n}); err != nil {
			return nil, fmt.Errorf("bad -view-omit %q: %v", n, err)
		}
	}
	return names, nil
}

// runListen serves one agent per accepted connection on a Unix socket. An
// existing socket path is refused, not replaced.
func runListen(listen string, cfg refagent.Config, stderr io.Writer) int {
	path, ok := strings.CutPrefix(listen, "unix:")
	if !ok {
		fmt.Fprintf(stderr, "extagent: -listen must be unix:<path>, got %q\n", listen)
		return 2
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "extagent: socket path %q already exists\n", path)
		return 2
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		fmt.Fprintf(stderr, "extagent: listen %s: %v\n", listen, err)
		return 1
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(stderr, "extagent: accept: %v\n", err)
			return 1
		}
		go func() {
			defer conn.Close()
			if err := refagent.Serve(conn, conn, cfg); err != nil {
				fmt.Fprintf(stderr, "extagent: connection: %v\n", err)
			}
		}()
	}
}
