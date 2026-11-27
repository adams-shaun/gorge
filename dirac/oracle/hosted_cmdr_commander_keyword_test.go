package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// hostedCmdrWindgrace plays Lord Windgrace ("CARDNAME can be your commander.",
// a planeswalker commander) as seat 0's commander against a bears deck. The
// keyword is deck-construction eligibility with no game characteristic, so
// kgen carries nothing for it; the shape that must match is the commander
// cast from the command zone and the loyalty activations. The case fails
// unless some seed casts Windgrace and activates a loyalty ability.
func hostedCmdrWindgrace(t *testing.T) {
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	lib0 := append(append(deckOf(7, "Swamp"), deckOf(7, "Mountain")...), append(deckOf(7, "Forest"), deckOf(19, "Grizzly Bears")...)...)
	deck0 := append([]string{"Lord Windgrace"}, lib0...)
	deck1 := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	names := [2][]string{deck0, deck1}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s, ns := range names {
		for _, n := range ns {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
			ents[s] = append(ents[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	set := printedDelveSetIn(t, corpusDirOrSkip(t), names)
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	cast, activated := 0, 0
	for seed := uint64(1); seed <= 40 && activated < 3; seed++ {
		cfg := Config{}
		m, err := NewMatchup(reg, [2]string{"windgrace-a", "windgrace-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.Commander = true
		m.Commanders, m.Partners = [2]int{0, -1}, [2]int{-1, -1}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		played := map[string]int{}
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), rc)
		CloseSide(k)
		if n := played["Lord Windgrace"]; n > 0 {
			cast++
			if n > 1 {
				activated++ // the cast plus a loyalty activation
			}
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
	}
	if cast == 0 || activated == 0 {
		t.Fatalf("Windgrace cast in %d games, activated in %d: the lockstep is vacuous", cast, activated)
	}
}
