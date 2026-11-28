package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

func censusCorpus(t *testing.T) *cards.Registry {
	t.Helper()
	dir, err := cardset.CorpusDir()
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no corpus: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// readTSV reads a census table: its header and rows keyed by column name.
func readTSV(t *testing.T, path string) (string, []map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	cols := strings.Split(lines[0], "\t")
	var rows []map[string]string
	for _, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) != len(cols) {
			t.Fatalf("%s: %d fields, want %d: %q", path, len(f), len(cols), l)
		}
		m := map[string]string{}
		for i, c := range cols {
			m[c] = f[i]
		}
		rows = append(rows, m)
	}
	return lines[0], rows
}

func runCensusTo(t *testing.T, reg *cards.Registry, dir string) (string, string) {
	t.Helper()
	out := t.TempDir()
	var stdout bytes.Buffer
	if err := census(reg, dir, out, 50, &stdout); err != nil {
		t.Fatal(err)
	}
	return out, stdout.String()
}

// TestDeckCensus: over the repo's deck files, one decks.tsv row per
// Commander deck, every exact deck free of blockers and every inexact legal
// one carrying at least one, sub3 within 3 substituted copies, the summary
// line counting the rows, and a rerun writing identical bytes.
func TestDeckCensus(t *testing.T) {
	reg := censusCorpus(t)
	want := map[string]bool{}
	paths, err := filepath.Glob(filepath.Join(deckDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if f, err := deck.Parse(raw); err == nil && len(f.CommanderNames()) > 0 {
			want[deck.Stem(p)] = true
		}
	}
	if len(want) == 0 || len(want) == len(paths) {
		t.Fatalf("precondition: %d of %d deck files are Commander decks; want some but not all", len(want), len(paths))
	}
	out, stdout := runCensusTo(t, reg, deckDir)
	dh, decks := readTSV(t, filepath.Join(out, "decks.tsv"))
	bh, blockers := readTSV(t, filepath.Join(out, "blockers.tsv"))
	if dh != "deck_id\tcards\tnonbasic\tlegal\texact\tsub3\tblock_cards\tblock_copies\tnot_lowered\tunverified\trefused\ttwo_seat\tinteraction\tnot_in_corpus\tsubstitute\terror" {
		t.Fatalf("decks.tsv header %q", dh)
	}
	if bh != "deck_id\tcard\tcopies\tcommander\tstatus\tclass\tclasses\treason" {
		t.Fatalf("blockers.tsv header %q", bh)
	}
	if len(decks) != len(want) {
		t.Fatalf("decks.tsv has %d rows, want %d", len(decks), len(want))
	}
	nBlock := map[string]int{}
	for _, b := range blockers {
		nBlock[b["deck_id"]]++
	}
	legal, exact, sub3 := 0, 0, 0
	for _, d := range decks {
		id := d["deck_id"]
		if !want[id] {
			t.Errorf("decks.tsv row %q is not a Commander deck file", id)
		}
		copies, err := strconv.Atoi(d["block_copies"])
		if err != nil {
			t.Fatalf("%s: block_copies %q", id, d["block_copies"])
		}
		switch {
		case d["exact"] == "1" && nBlock[id] != 0:
			t.Errorf("%s: exact with %d blocker rows", id, nBlock[id])
		case d["exact"] == "0" && d["legal"] == "yes" && nBlock[id] == 0:
			t.Errorf("%s: legal, not exact, and no blocker row (error %q)", id, d["error"])
		}
		if d["sub3"] == "1" && copies > censusSub3 {
			t.Errorf("%s: sub3 with %d blocking copies", id, copies)
		}
		if d["legal"] == "yes" {
			legal++
		}
		if d["exact"] == "1" {
			exact++
		}
		if d["sub3"] == "1" {
			sub3++
		}
	}
	prefix := fmt.Sprintf("census: decks=%d legal=%d exact=%d sub3=%d ", len(decks), legal, exact, sub3)
	if !strings.HasPrefix(stdout, prefix) {
		t.Fatalf("stdout %q, want prefix %q", stdout, prefix)
	}
	out2, stdout2 := runCensusTo(t, reg, deckDir)
	if stdout2 != stdout {
		t.Errorf("rerun stdout %q, first %q", stdout2, stdout)
	}
	for _, name := range []string{"decks.tsv", "blockers.tsv"} {
		a, _ := os.ReadFile(filepath.Join(out, name))
		b, _ := os.ReadFile(filepath.Join(out2, name))
		if !bytes.Equal(a, b) {
			t.Errorf("rerun %s differs", name)
		}
	}
	t.Logf("%s(exact %d of %d decks)", stdout, exact, len(decks))
}

// TestDeckCensusNotInCorpus: a card the corpus does not hold makes the deck's
// legality unknown, the deck inexact, and the card a not_in_corpus blocker.
func TestDeckCensusNotInCorpus(t *testing.T) {
	reg := censusCorpus(t)
	if _, ok := reg.Lookup("No Such Card"); ok {
		t.Fatal("precondition: No Such Card is in the corpus")
	}
	dir := t.TempDir()
	src := `{"name":"ghost","format":"commander","commander":"Sire of Seven Deaths","cards":[
{"name":"Sire of Seven Deaths","count":1},{"name":"No Such Card","count":1},{"name":"Swamp","count":98}]}`
	if err := os.WriteFile(filepath.Join(dir, "ghost.json"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := runCensusTo(t, reg, dir)
	_, decks := readTSV(t, filepath.Join(out, "decks.tsv"))
	_, blockers := readTSV(t, filepath.Join(out, "blockers.tsv"))
	if len(decks) != 1 || decks[0]["deck_id"] != "ghost" {
		t.Fatalf("decks.tsv rows %v, want the one ghost row", decks)
	}
	if d := decks[0]; d["legal"] != "unknown" || d["exact"] != "0" || d["not_in_corpus"] != "1" {
		t.Errorf("ghost row %v: want legal=unknown exact=0 not_in_corpus=1", d)
	}
	found := false
	for _, b := range blockers {
		if b["card"] == "No Such Card" {
			found = true
			if b["status"] != "not_in_corpus" || b["class"] != "status:not_in_corpus" || b["copies"] != "1" {
				t.Errorf("No Such Card blocker %v", b)
			}
		}
	}
	if !found {
		t.Errorf("no blocker row for No Such Card: %v", blockers)
	}
}
