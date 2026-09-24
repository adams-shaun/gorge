package main

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

func TestSignatureCollapsesSeedNumbers(t *testing.T) {
	a := signature("livelock", "livelock detected (repeating cycle): object 12, kind priority, cycle: [seq 40 priority player=1]")
	b := signature("livelock", "livelock detected (repeating cycle): object 99, kind priority, cycle: [seq 7123 priority player=0]")
	if a != b {
		t.Fatalf("same cycle on different seeds must share a signature:\n%s\n%s", a, b)
	}
}

func TestGenerateIsDeterministicMonoColourSixty(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := loadCov(t.TempDir() + "/none.json")
	gen := func() genDeck { return generate(rand.New(rand.NewPCG(7, 9)), p, c) }
	d1, d2 := gen(), gen()
	if len(d1.Cards) != 60 {
		t.Fatalf("deck has %d cards, want 60", len(d1.Cards))
	}
	for i := range d1.Cards {
		if d1.Cards[i] != d2.Cards[i] {
			t.Fatalf("generation not deterministic at %d: %s vs %s", i, d1.Cards[i], d2.Cards[i])
		}
	}
	cards, err := resolveDeck(reg, d1)
	if err != nil {
		t.Fatal(err)
	}
	lands := 0
	ci := -1
	for i, n := range colourNames {
		if n == d1.Colour {
			ci = i
		}
	}
	for _, cd := range cards {
		if cd.Faces[0].IsLand() {
			lands++
		}
		if id := identity(cd); id != 0 && id != 1<<ci {
			t.Fatalf("%s has identity %b outside mono-%s", cardName(cd), id, d1.Colour)
		}
	}
	if lands < 20 {
		t.Fatalf("deck has %d lands, want at least 20", lands)
	}
}

// TestBoardGuardRecordsBigboard pins the harness-side board-size watchdog:
// a game whose live object count exceeds -max-objects ends as its own
// "bigboard" kind (never "hang" or "livelock", so triage can tell a runaway
// token engine from an engine bug), and the cap is inert below the limit
// and when disabled. Two 60-card decks put 120 live objects in the arena at
// genesis, so a cap of 100 fires at the first decision and one of 100000
// never does.
func TestBoardGuardRecordsBigboard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := loadCov(t.TempDir() + "/none.json")
	r := rand.New(rand.NewPCG(3, 5))
	decks := []genDeck{generate(r, p, c), generate(r, p, c)}

	f, _, _ := playOne(reg, decks, 11, 3, 20000, 100, false)
	if f == nil || f.Kind != "bigboard" {
		t.Fatalf("failure = %+v, want kind bigboard", f)
	}
	if !strings.Contains(f.Diag, "exceeds -max-objects 100") || !strings.HasPrefix(f.Sig, "bigboard: ") {
		t.Fatalf("bigboard record lacks its diagnostic: sig %q diag %q", f.Sig, f.Diag)
	}
	if boardGuard(0) != nil {
		t.Fatalf("-max-objects 0 must disable the guard")
	}
	// Under the cap the guard is inert: the 3-turn cap ends the game as a
	// plain stall (not a failure record).
	if f, _, _ := playOne(reg, decks, 11, 3, 20000, 100000, false); f != nil {
		t.Fatalf("game under the object cap recorded %+v", f)
	}
}
