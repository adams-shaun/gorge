package main

import (
	"math/rand/v2"
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
