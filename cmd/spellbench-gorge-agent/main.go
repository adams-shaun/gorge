// Command spellbench-gorge-agent serves gorge's registered bot policies in the
// Spellbench v2 agent role over stdio.
//
// Ported from github.com/jackmaiorino/spellbench/engines/gorge/cmd/spellbench-gorge-agent
// at revision 1beea4c104911bba77a5672c644e570ed9afe28a (MIT).
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
)

func main() {
	policy := flag.String("policy", "bot", "registered bot policy key")
	corpus := flag.String("corpus", os.Getenv("GORGE_CARDS"), "pinned corpus directory (neutral world)")
	registry := flag.String("registry", "", "frozen registry file (neutral world)")
	registrySHA := flag.String("registry-sha256", "", "expected SHA-256 of the frozen registry")
	auditDir := flag.String("audit-dir", os.Getenv("GORGE_AGENT_AUDIT_DIR"), "optional directory for aggregate policy coverage after the process closes")
	world := flag.String("world", "gorge", "gorge (read x_gorge_view_v1 from gorge's engine) or neutral (translate the v2 observation and candidates of any engine; needs the registry)")
	listPolicies := flag.Bool("list-policies", false, "list registered policies and exit")
	flag.Parse()
	if *listPolicies {
		for _, name := range bots.Names() {
			e, ok := bots.Lookup(name)
			if !ok {
				fmt.Fprintf(os.Stderr, "spellbench-gorge-agent: registry names %q but Lookup misses it\n", name)
				os.Exit(1)
			}
			fmt.Printf("%s\t%s\t%s\n", e.Name, e.Label, e.Tier)
		}
		return
	}
	s, err := agent.New(*policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch *world {
	case "gorge":
	case "neutral":
		reg, err := openInput(*corpus, *registry, *registrySHA)
		if err != nil {
			fmt.Fprintln(os.Stderr, "spellbench-gorge-agent:", err)
			os.Exit(2)
		}
		s.SetRegistry(reg)
		if err := s.EnableNeutral(); err != nil {
			fmt.Fprintln(os.Stderr, "spellbench-gorge-agent:", err)
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "spellbench-gorge-agent: unknown world %q\n", *world)
		os.Exit(2)
	}
	err = agent.Serve(os.Stdin, &flushWriter{bufio.NewWriter(os.Stdout)}, s)
	if *auditDir != "" {
		if auditErr := writeAudit(*auditDir, s.Audit()); auditErr != nil {
			fmt.Fprintln(os.Stderr, "spellbench-gorge-agent audit:", auditErr)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "spellbench-gorge-agent:", err)
		os.Exit(1)
	}
}

func openInput(corpus, registry, wantSHA string) (*cards.Registry, error) {
	if registry != "" {
		if wantSHA == "" {
			return nil, fmt.Errorf("registry-sha256 is required with registry")
		}
		want, err := hex.DecodeString(wantSHA)
		if err != nil || len(want) != sha256.Size {
			return nil, fmt.Errorf("registry-sha256 must be a SHA-256 digest")
		}
		b, err := os.ReadFile(registry)
		if err != nil {
			return nil, err
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != hex.EncodeToString(want) {
			return nil, fmt.Errorf("frozen registry SHA-256 mismatch")
		}
		return cards.LoadRegistry(registry)
	}
	if wantSHA != "" {
		return nil, fmt.Errorf("registry-sha256 requires registry")
	}
	if corpus == "" {
		return nil, fmt.Errorf("no corpus: pass -corpus or set GORGE_CARDS")
	}
	return cards.OpenCorpus(corpus)
}

func writeAudit(dir string, value agent.PolicyAudit) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "gorge-agent-*.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(value); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type flushWriter struct{ w *bufio.Writer }

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil {
		err = f.w.Flush()
	}
	return n, err
}
