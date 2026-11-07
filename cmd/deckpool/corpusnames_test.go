package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDeckpoolWritesCorpusNames holds a deck file to the printed names the corpus
// and the dirac tables use, not the draft data's spelling.
func TestDeckpoolWritesCorpusNames(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	const draftName = "All Good Things..."
	const corpusName = "All Good Things . . ."
	lab, err := draftlab.Open(corpus, draftlab.Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := lab.Card(draftName)
	if !ok {
		t.Fatalf("precondition: the lab cannot resolve the draft name %q", draftName)
	}
	if len(c.Faces) == 0 || c.Faces[0].Name != corpusName {
		t.Fatalf("precondition: %q resolves to %q, want %q", draftName, c.Faces[0].Name, corpusName)
	}
	spec, err := setdata.Load("TRK")
	if err != nil {
		t.Fatal(err)
	}
	foundDraftName := false
	for _, sheet := range spec.Config.Sheets {
		for _, card := range sheet.Cards {
			if card.Name == draftName {
				foundDraftName = true
			}
		}
	}
	if !foundDraftName {
		t.Fatalf("precondition: the TRK draft data does not name %q", draftName)
	}

	// Which pod plays the card depends on the pick ratings and the builder, so
	// the test walks a fixed run of seeds and every deck each one writes: no deck
	// may carry the draft spelling, and some deck must carry the corpus spelling,
	// or the run proved nothing.
	for seed := uint64(1015); seed < 1015+12; seed++ {
		if deckpoolHolds(t, corpus, seed, draftName, corpusName) {
			return
		}
	}
	t.Fatalf("no deck from seeds 1015..1026 holds %q", corpusName)
}

// deckpoolHolds runs one TRK pod at seed, fails the test on any deck holding the
// draft spelling, and reports whether some deck holds the corpus spelling.
func deckpoolHolds(t *testing.T, corpus string, seed uint64, draftName, corpusName string) bool {
	t.Helper()
	out := filepath.Join(t.TempDir(), "pool")
	o := options{set: "TRK", pods: 1, seed: seed, drafter: "balanced", builder: "balanced", corpus: corpus, out: out, quiet: true}
	if err := run(o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(out, "decks", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("seed %d wrote no deck files", seed)
	}
	held := false
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var f deck.File
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, e := range f.Cards {
			switch e.Name {
			case draftName:
				t.Errorf("%s still holds the draft spelling %q", filepath.Base(p), draftName)
			case corpusName:
				held = true
			}
		}
	}
	return held
}
