package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

	out := filepath.Join(t.TempDir(), "pool")
	o := options{set: "TRK", pods: 1, seed: 1015, drafter: "balanced", builder: "balanced", corpus: corpus, out: out, quiet: true}
	if err := run(o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "decks", "TRK-1015-s1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f deck.File
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(f.Cards))
	for i, e := range f.Cards {
		names[i] = e.Name
	}
	if !slices.Contains(names, corpusName) {
		t.Fatalf("deck holds %v, want %q", names, corpusName)
	}
	if slices.Contains(names, draftName) {
		t.Fatalf("deck still holds the draft spelling %q: %v", draftName, names)
	}
}
