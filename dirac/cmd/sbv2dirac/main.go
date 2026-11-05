// Command sbv2dirac serves the engine role of SpellBench v2 with the dirac
// kernel as the authority for game flow, reading NDJSON requests on stdin and
// writing responses on stdout. Package sbv2 documents the protocol and its
// lockstep checks.
//
//	go run ./dirac/cmd/sbv2dirac -corpus .cards
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cmd/sbv2dirac/sbv2"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/env"
)

func main() {
	corpus := flag.String("corpus", "", "compiled Forge corpus directory (default: GORGE_CARDS or .cards)")
	decks := flag.String("decks", "", "directory of gorge deck JSON files to serve beside the catalog (catalog_id = file stem)")
	rev := flag.String("source-revision", "", "adapter source revision reported in hello_ok")
	flag.Parse()
	if err := run(*corpus, *decks, *rev); err != nil {
		fmt.Fprintln(os.Stderr, "sbv2dirac:", err)
		os.Exit(2)
	}
}

func run(corpus, decks, rev string) error {
	if corpus == "" {
		var err error
		if corpus, err = cardset.CorpusDir(); err != nil {
			return err
		}
	} else {
		os.Setenv("GORGE_CARDS", corpus) // the tables come from the corpus this run plays, not the one the cwd happens to sit under
	}
	if lock, err := cards.ReadLock(corpus); err == nil {
		env.SetCorpusRef(lock.Commit)
	}
	reg, err := cards.OpenCorpus(corpus)
	if err != nil {
		return err
	}
	var sr *string
	if rev != "" {
		sr = &rev
	}
	srv, err := sbv2.New(reg, sr)
	if err != nil {
		return err
	}
	defer srv.Close()
	if decks != "" {
		files, err := filepath.Glob(filepath.Join(decks, "*.json"))
		if err != nil {
			return err
		}
		sort.Strings(files)
		for _, f := range files {
			d, err := sbv2.ReadDeck(f)
			if err != nil {
				return err
			}
			srv.AddDeck(d)
		}
	}
	out := bufio.NewWriter(os.Stdout)
	return sbv2.Serve(os.Stdin, &flushWriter{out}, srv)
}

type flushWriter struct{ w *bufio.Writer }

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil {
		err = f.w.Flush()
	}
	return n, err
}
