package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// hostedCmdrNecrodominance plays Necrodominance ("at the beginning of your end
// step, you may pay any amount of life. If you do, draw that many cards"; a
// trigger's own PayLife<X> cost, SVar X Count$xPaid) in both engines. gorge
// announces X over 0..life, then offers the pay election, then the body draws
// X; the kernel's PayManaX/PayBSubLifeX cost (cost_paylifetrig.go) must pose
// the same asks. The case fails unless some game pays at least one life
// through the trigger (a Draw event that is not the draw step's).
func hostedCmdrNecrodominance(t *testing.T) {
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	deck0 := append(append(deckOf(10, "Necrodominance"), deckOf(16, "Swamp")...), deckOf(14, "Grizzly Bears")...)
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
	paid, games := 0, 0
	for seed := uint64(1); seed <= 40 && paid < 3; seed++ {
		cfg := Config{}
		m, err := NewMatchup(reg, [2]string{"necrodominance-a", "necrodominance-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30}
		rc.Finish = func(g *GorgeSide) {
			// seat 0 skips its draw step under Necrodominance, so its life
			// lost to a trigger cost is the only way it loses life on its own
			// turn here: count LifeChange losses of seat 0 followed by a draw
			for i, e := range g.E.L.Events {
				if e.Kind == events.LifeChange && e.Player == 0 && e.Amount < 0 {
					for _, n := range g.E.L.Events[i+1 : min(i+4, len(g.E.L.Events))] {
						if n.Kind == events.Draw && n.Player == 0 {
							paid++
							return
						}
					}
				}
			}
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, map[string]int{}), rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
	}
	if paid == 0 {
		t.Fatalf("%d games: none paid life through the trigger and drew, so the lockstep is vacuous", games)
	}
}
