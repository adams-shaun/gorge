package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// listAFixture writes the slice's first two pair lines alternated 10x to a temp file and
// returns the options that play it, the resolved pairs and the decks directory.
func listAFixture(t *testing.T) (o *options, ps []oracle.Pairing, decksDir string) {
	t.Helper()
	dir := sliceDecks(t)
	raw, err := os.ReadFile(filepath.Join(dir, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the slice's pairs.tsv has %d lines, want at least 2", len(lines))
	}
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		buf.WriteString(lines[0])
		buf.WriteString("\n")
		buf.WriteString(lines[1])
		buf.WriteString("\n")
	}
	list := filepath.Join(t.TempDir(), "pairs.tsv")
	if err := os.WriteFile(list, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	o = &options{set: "fdn", a: seatRandom, b: seatRandom, mullPolicy: "coin", maxTurns: 100, maxIntents: 20000,
		seed: oracle.SliceSeed, pairsFile: list, decks: filepath.Join(dir, "decks")}
	ps, decksDir, remove, err := o.dealtPairs()
	t.Cleanup(remove)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 20 {
		t.Fatalf("%d pairs from the alternating list, want 20", len(ps))
	}
	return o, ps, decksDir
}

// soloRow plays p alone on its own runner: what one pair's game is with nothing cached.
func soloRow(t *testing.T, o *options, p *oracle.Pairing, decks map[string]*deckEntry, decksDir string) row {
	t.Helper()
	r, err := newRunner(o, cardsettest.Set(t, "fdn"), []oracle.Pairing{*p}, decks, decksDir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	w := r.newWorker()
	var got row
	w.play(p, &got)
	if got.err != "" || got.refused != "" {
		t.Fatalf("pair %d: %+v", p.Index, got)
	}
	return got
}

// rowsDiffer reports whether two rows say different games.
func rowsDiffer(a, b row) bool {
	return a.winner != b.winner || a.turns != b.turns || a.intents != b.intents || a.stall != b.stall
}

// rowEQ fails t when a and b differ in the fields the summary reports.
func rowEQ(t *testing.T, what string, a, b row) {
	t.Helper()
	if a.winner != b.winner || a.turns != b.turns || a.intents != b.intents || a.stall != b.stall {
		t.Fatalf("%s: winner %d turns %d intents %d stall %q, want winner %d turns %d intents %d stall %q",
			what, a.winner, a.turns, a.intents, a.stall, b.winner, b.turns, b.intents, b.stall)
	}
}

// TestDplayPairTableCacheReused plays an alternating list of the slice's first two pair
// lines and checks the runner builds each pair's base once, with every game identical to
// its pair's solo run, serial and on two workers.
func TestDplayPairTableCacheReused(t *testing.T) {
	o, ps, decksDir := listAFixture(t)
	set := cardsettest.Set(t, "fdn")
	decks, err := readDecks(set, ps, decksDir, false)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the two lines name four different decks, and their games differ.
	names := map[string]bool{}
	for _, p := range ps[:2] {
		names[p.A], names[p.B] = true, true
	}
	if len(names) != 4 {
		t.Fatalf("the two pair lines name %d decks, want 4 distinct: %v", len(names), names)
	}
	ref0, ref1 := soloRow(t, o, &ps[0], decks, decksDir), soloRow(t, o, &ps[1], decks, decksDir)
	if !rowsDiffer(ref0, ref1) {
		t.Fatalf("the two pairs' solo games do not differ: %+v vs %+v", ref0, ref1)
	}

	// Serial run over the alternating list: two fills, and every game matches its pair's
	// solo run.
	r, err := newRunner(o, set, ps, decks, decksDir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	w := r.newWorker()
	var got [20]row
	for i := range ps {
		w.play(&ps[i], &got[i])
		if got[i].err != "" || got[i].refused != "" {
			t.Fatalf("pair %d: %+v", ps[i].Index, got[i])
		}
	}
	if r.built != 2 {
		t.Fatalf("built %d bases over 20 alternating games, want 2", r.built)
	}
	for i := range ps {
		want := ref0
		if i%2 == 1 {
			want = ref1
		}
		rowEQ(t, "serial pair "+ps[i].A+" vs "+ps[i].B, got[i], want)
	}

	// Two workers over the same list: same fills, same games.
	o2 := *o
	o2.workers = 2
	r2, err := newRunner(&o2, set, ps, decks, decksDir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	k := &sink{w: bufio.NewWriterSize(&buf, 1<<16), held: map[int]*row{},
		sum: summary{Set: o.set, Config: o.config, Seats: [2]string{o.a, o.b}, StallsByClass: map[string]map[string]int{}}}
	if err := r2.playAll(k); err != nil {
		t.Fatal(err)
	}
	if err := k.w.Flush(); err != nil {
		t.Fatal(err)
	}
	if r2.built != 2 {
		t.Fatalf("two workers built %d bases over 20 alternating games, want 2", r2.built)
	}
	sc := bufio.NewScanner(&buf)
	seen := 0
	for sc.Scan() {
		var line struct {
			Pair    int    `json:"pair"`
			Winner  int    `json:"winner"`
			Turns   int32  `json:"turns"`
			Intents int32  `json:"intents"`
			Stall   string `json:"stall"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			t.Fatalf("game line %d: %v", seen, err)
		}
		rowEQ(t, "workers pair line", row{winner: int8(line.Winner), turns: line.Turns, intents: line.Intents, stall: line.Stall}, got[line.Pair])
		seen++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != 20 {
		t.Fatalf("%d worker games, want 20", seen)
	}
}
