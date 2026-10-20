// Command spellbench-gorge-env serves the Spellbench v2 environment role over stdio.
//
// Ported from github.com/jackmaiorino/spellbench/engines/gorge/cmd/spellbench-gorge-env
// at revision 1beea4c104911bba77a5672c644e570ed9afe28a (MIT).
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
)

func main() {
	corpus := flag.String("corpus", os.Getenv("GORGE_CARDS"), "compiled Forge corpus directory (never shipped)")
	registry := flag.String("registry", "", "frozen registry file (reproducible input binding)")
	registrySHA := flag.String("registry-sha256", "", "expected SHA-256 of the frozen registry")
	rev := flag.String("source-revision", "", "adapter source revision reported in hello_ok")
	autoPay := flag.Bool("auto-pay", true, "pay mana with the engine planner (the default; accepted for compatibility)")
	manualMana := flag.Bool("manual-mana", false, "offer interactive mana taps instead of engine auto-pay")
	flag.Parse()
	reg, err := openInput(*corpus, *registry, *registrySHA)
	if err != nil {
		fmt.Fprintln(os.Stderr, "spellbench-gorge-env:", err)
		os.Exit(2)
	}
	if *corpus != "" {
		if lock, err := cards.ReadLock(*corpus); err == nil {
			env.SetCorpusRef(lock.Commit)
		}
	}
	if err := catalog.Preflight(reg); err != nil {
		fmt.Fprintln(os.Stderr, "spellbench-gorge-env:", err)
		os.Exit(2)
	}
	var sr *string
	if *rev != "" {
		sr = rev
	}
	out := bufio.NewWriter(os.Stdout)
	w := &flushWriter{out}
	srv := env.New(reg, sr)
	if *autoPay && !*manualMana {
		srv.EnableAutoPay()
	}
	if err := env.Serve(os.Stdin, w, srv); err != nil {
		fmt.Fprintln(os.Stderr, "spellbench-gorge-env:", err)
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

type flushWriter struct{ w *bufio.Writer }

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil {
		err = f.w.Flush()
	}
	return n, err
}
