package testutil

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
)

// TestFoundationsCommanderDecksAreOfficial pins the five foundations-* decks to
// the official Foundations Commander (FDC) lists: neither Karakas (banned in
// Commander, which the interim Calling All Angels held) nor Ancient Tomb (never
// printed in FDC, which an interim list had to replace) is in any of them, and
// no deck's notes still call the list interim. The names are asserted against
// the corpus first so the absence check is about cards the registry knows.
func TestFoundationsCommanderDecksAreOfficial(t *testing.T) {
	reg := CorpusRegistry(t)
	absent := []string{"Karakas", "Ancient Tomb"}
	for _, n := range absent {
		if _, ok := reg.Lookup(n); !ok {
			t.Fatalf("precondition: %q is not in the corpus, so its absence from a deck proves nothing", n)
		}
	}
	for _, rcd := range repoCommanderDecks {
		if !strings.HasPrefix(rcd.file, "foundations-") {
			continue
		}
		file := rcd.file
		t.Run(file, func(t *testing.T) {
			f := RepoDeckFile(t, file)
			total := 0
			for _, e := range f.Cards {
				total += e.Count
				for _, n := range absent {
					if e.Name == n {
						t.Errorf("%s lists %q", file, n)
					}
				}
			}
			if total != 100 {
				t.Fatalf("precondition: %s has %d cards, want 100", file, total)
			}
			raw, err := decksFS.ReadFile(path.Join(decksDir, file+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var meta struct {
				Notes string `json:"notes"`
			}
			if err := json.Unmarshal(raw, &meta); err != nil {
				t.Fatal(err)
			}
			if meta.Notes == "" {
				t.Fatalf("precondition: %s has no notes to check", file)
			}
			if strings.Contains(meta.Notes, "nterim") {
				t.Errorf("%s notes still say interim: %q", file, meta.Notes)
			}
		})
	}
}
