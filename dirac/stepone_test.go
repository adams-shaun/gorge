package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStepOneIsStepInto pins StepOne's contract: stepping with StepOne and
// answering every decision Elides reports with Elide poses exactly
// StepInto's decisions (kind, player, options in order) and leaves the
// State StepInto leaves, elided decisions and Skipped included. self-play
// -declog steps this way to see the elided decisions, and its games must
// be the games self-play plays without it.
func TestStepOneIsStepInto(t *testing.T) {
	play := func(t *testing.T, name string, tbl *kop.Table, a, b []CardID, seed uint64, cfg Config) int {
		t.Helper()
		s1 := NewConfig(tbl, a, b, seed, cfg)
		s2 := new(State)
		s2.CopyFrom(s1)
		p1, p2 := NewRandomPolicy(seed*7919+3), NewRandomPolicy(seed*7919+3)
		var d1, d2 Decision
		elided := 0
		for n := 0; n < 30000; n++ {
			s1.StepInto(&d1)
			for {
				s2.StepOne(&d2)
				if !s2.Elides(&d2) {
					break
				}
				elided++
				s2.Elide(&d2)
			}
			if d1.Kind != d2.Kind || d1.Sub != d2.Sub || d1.Player != d2.Player || d1.N != d2.N || !sameOptions(&d1, &d2) {
				t.Fatalf("%s: decision %d differs: StepInto %d/%d p%d %08x, StepOne %d/%d p%d %08x", name, n,
					d1.Kind, d1.Sub, d1.Player, d1.Options[:d1.N], d2.Kind, d2.Sub, d2.Player, d2.Options[:d2.N])
			}
			if d1.Kind == DkNone {
				break
			}
			x, y := p1.Pick(&d1), p2.Pick(&d2)
			s1.Apply(x)
			s2.Apply(y)
		}
		if !s1.SameGame(s2) {
			t.Fatalf("%s: the States differ at the end (Skipped %v vs %v, decisions %d vs %d)", name, s1.Skipped, s2.Skipped,
				s1.SeqNo(), s2.SeqNo())
		}
		if cfg.NoElide && elided != 0 {
			t.Fatalf("%s: %d decisions elided with NoElide", name, elided)
		}
		return elided
	}
	elided := 0
	for g := 0; g < 40; g++ {
		a, b := vanilla.Decks[g%4], vanilla.Decks[(g/4)%4]
		cfg := Config{Mulligans: uint8(g % 3), NoElide: g%5 == 4}
		elided += play(t, "vanilla", vanilla.Table, a, b, uint64(g)+1, cfg)
	}
	if elided == 0 {
		t.Fatal("no vanilla decision was elided: the test compares nothing StepInto answers in place")
	}
	t.Run("fdn", func(t *testing.T) {
		decks := fdnDecks(t)
		n := 0
		set := cardsettest.Set(t, "fdn")
		for g := 0; g < 12; g++ {
			tbl, da, db, err := set.Matchup(decks[(g*7919)%len(decks)], decks[(g*104729+31)%len(decks)])
			if err != nil {
				t.Fatal(err)
			}
			n += play(t, "fdn", tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3)})
			ReleaseTable(tbl)
		}
		if n == 0 {
			t.Fatal("no FDN decision was elided")
		}
	})
}

func sameOptions(a, b *Decision) bool {
	for i := 0; i < int(a.N); i++ {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}
