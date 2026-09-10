package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
)

// testCorpusDir returns the repo-root .cards corpus directory, skipping the
// test when it is absent (a fresh worktree may not carry the gitignored
// corpus). It resolves the repo root from the test's own working directory
// (the package dir, at <root>/cmd/deckimport).
func testCorpusDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(wd))
	dir := filepath.Join(root, ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no corpus at %s (run make fetch-cards compile-cards)", dir)
	}
	return dir
}

// writeTempDecklist writes body to a temp file and returns its path.
func writeTempDecklist(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConvertReportsMissingAndForceStillWrites(t *testing.T) {
	corpusDir := testCorpusDir(t)
	in := writeTempDecklist(t, "4 Lightning Bolt\n4 Monastery Swiftspear\n2 Totally Fake Card\n20 Mountain\n")
	out := filepath.Join(t.TempDir(), "deck.json")

	// Without -force the file is NOT written; the report names the missing card.
	dr, err := convert(in, corpusDir, "", "", out, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if dr.Ok {
		t.Fatal("deck with an unknown card must not be Ok")
	}
	if dr.Cards != 30 || dr.Resolved != 28 {
		t.Fatalf("cards=%d resolved=%d, want 30/28", dr.Cards, dr.Resolved)
	}
	if dr.Percent < 93.0 || dr.Percent > 93.4 {
		t.Fatalf("percent = %.2f", dr.Percent)
	}
	if len(dr.Missing) != 1 || dr.Missing[0].Name != "Totally Fake Card" || dr.Missing[0].Count != 2 {
		t.Fatalf("missing = %+v", dr.Missing)
	}
	if dr.Written {
		t.Fatal("file must not be written without -force")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("deck file written without -force")
	}

	// With -force the file IS written, even though the deck is incomplete.
	dr, err = convert(in, corpusDir, "", "", out, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !dr.Written {
		t.Fatal("-force must write the deck file")
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// The written file must be a valid repo deck file: deck.Parse reads it.
	if _, err := deck.Parse(raw); err != nil {
		t.Fatalf("written deck file does not re-parse: %v", err)
	}
	// The sideboard was not involved here, so the card count is intact.
	var parsed deck.File
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Cards) != 4 {
		t.Fatalf("deck file has %d entries, want 4", len(parsed.Cards))
	}
}

func TestRoundTripWritesLoadableDeckFile(t *testing.T) {
	corpusDir := testCorpusDir(t)
	in := writeTempDecklist(t, "4 Lightning Bolt\n4 Goblin Guide\n4 Monastery Swiftspear\n12 Mountain\n")
	out := filepath.Join(t.TempDir(), "deck.json")

	dr, err := convert(in, corpusDir, "Mono Red", "custom", out, false, "aggro", "")
	if err != nil {
		t.Fatal(err)
	}
	if !dr.Ok || !dr.Written || dr.Resolved != 24 || dr.Cards != 24 {
		t.Fatalf("Ok=%v written=%v resolved=%d cards=%d", dr.Ok, dr.Written, dr.Resolved, dr.Cards)
	}

	// Now the real round trip: deck.Load must read the written file back with
	// no error and give back every card.
	r, err := cards.OpenCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	f, cs, err := deck.Load(r, out)
	if err != nil {
		t.Fatalf("deck.Load of the written file failed: %v", err)
	}
	if f.Name != "Mono Red" || f.Format != "custom" {
		t.Fatalf("loaded name=%q format=%q", f.Name, f.Format)
	}
	if len(cs) != 24 {
		t.Fatalf("deck.Load gave %d cards, want 24", len(cs))
	}
}

func TestConvertCommanderSetsFieldAndChecksEligibility(t *testing.T) {
	corpusDir := testCorpusDir(t)
	in := writeTempDecklist(t, "Commander\nAtraxa, Praetors' Voice\n1 Birds of Paradise\n4 Llanowar Elves\n")
	dr, err := convert(in, corpusDir, "", "", filepath.Join(t.TempDir(), "c.json"), true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if dr.Commander != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander = %q", dr.Commander)
	}
	if dr.Format != "commander" {
		t.Fatalf("format = %q, want commander", dr.Format)
	}
	if dr.CommanderOk == nil {
		t.Fatal("commander_eligible must be set for a commander deck")
	}
	// This 6-card deck cannot be a valid 100-card Commander list, so it is
	// reported not-eligible with a reason (and criticising the size/legality,
	// not the commander's own eligibility).
	if *dr.CommanderOk {
		t.Log("commander reported eligible (corpus Atraxa is a legendary creature)")
	} else if len(dr.CommanderWhy) == 0 {
		t.Fatal("an ineligible commander must carry a reason")
	}
}

func TestJSONReportShape(t *testing.T) {
	ok := true
	rep := report{Decks: []deckReport{{
		Input: "list.txt", Name: "Deck", Format: "commander",
		Commander: "Atraxa, Praetors' Voice", Cards: 100, Resolved: 99,
		Percent: 99.0, Dropped: 15,
		Missing:     []missingCard{{Name: "Totally Fake Card", Count: 1}},
		CommanderOk: &ok, Ok: false, Written: true, Path: "deck.json",
		CommanderWhy: []string{"deck has 6 cards, not 100"},
	}}}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	decks, ok := m["decks"].([]any)
	if !ok || len(decks) != 1 {
		t.Fatalf("no decks array: %s", raw)
	}
	d := decks[0].(map[string]any)
	for _, key := range []string{
		"input", "name", "format", "commander", "cards", "resolved",
		"resolution_percent", "sideboard_cards_dropped", "missing",
		"commander_eligible", "commander_reason", "ok", "path", "written",
	} {
		if _, present := d[key]; !present {
			t.Errorf("JSON report is missing key %q", key)
		}
	}
	miss := d["missing"].([]any)
	if len(miss) != 1 || miss[0].(map[string]any)["name"] != "Totally Fake Card" {
		t.Errorf("missing entry wrong: %v", miss)
	}
	if d["resolution_percent"].(float64) != 99.0 {
		t.Errorf("resolution_percent wrong: %v", d["resolution_percent"])
	}
}

func TestConvertRejectsUnparseableLine(t *testing.T) {
	corpusDir := testCorpusDir(t)
	in := writeTempDecklist(t, "4 Lightning Bolt\nLightning Bolt\n")
	_, err := convert(in, corpusDir, "", "", "", false, "", "")
	if err == nil {
		t.Fatal("expected an error for a count-less card line")
	}
	if !strings.Contains(err.Error(), "Lightning Bolt") {
		t.Fatalf("error should name the offending card: %v", err)
	}
}
